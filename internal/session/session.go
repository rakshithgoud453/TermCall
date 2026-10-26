package session

import (
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rakshithgoud453/termcall/internal/audio"
	"github.com/rakshithgoud453/termcall/internal/codec"
	"github.com/rakshithgoud453/termcall/internal/crypto"
	"github.com/rakshithgoud453/termcall/internal/jitter"
	"github.com/rakshithgoud453/termcall/internal/network"
	"github.com/rakshithgoud453/termcall/pkg/protocol"
)

// Mode defines whether the session is Loopback, Host, or Join
type Mode int

const (
	ModeLoopback Mode = 1
	ModeHost     Mode = 2
	ModeJoin     Mode = 3
)

// Session coordinates audio hardware, Opus codec, cryptography, network transport, and jitter buffer.
type Session struct {
	mode         Mode
	audioEngine  *audio.Engine
	codec        *codec.Codec
	crypto       *crypto.CryptoSession
	keyPair      *crypto.KeyPair
	transport    *network.Transport
	jitterBuffer *jitter.JitterBuffer

	stateMu sync.RWMutex
	role    crypto.Role

	peerName     string
	peerEndpoint string
	roomToken    string

	// Control state
	ctx       context.Context
	cancel    context.CancelFunc
	running   atomic.Bool
	connected atomic.Bool

	hangupMu sync.Mutex
	onHangup func()
}

// NewLoopbackSession creates an end-to-end local loopback test session
func NewLoopbackSession() (*Session, error) {
	ctx, cancel := context.WithCancel(context.Background())

	audioEng, err := audio.NewEngine()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to init audio: %w", err)
	}

	opusCodec, err := codec.NewCodec()
	if err != nil {
		cancel()
		audioEng.Close()
		return nil, fmt.Errorf("failed to init opus codec: %w", err)
	}

	// Create ephemeral keypair and self-session
	keys, err := crypto.GenerateKeyPair()
	if err != nil {
		cancel()
		audioEng.Close()
		return nil, fmt.Errorf("failed to generate keys: %w", err)
	}

	// For loopback, Host role encrypts and decrypts with itself
	cryptoSess, err := crypto.NewCryptoSession(keys, keys.PublicKeyBytes(), crypto.RoleHost)
	if err != nil {
		cancel()
		audioEng.Close()
		return nil, fmt.Errorf("failed to init crypto session: %w", err)
	}

	s := &Session{
		mode:         ModeLoopback,
		audioEngine:  audioEng,
		codec:        opusCodec,
		crypto:       cryptoSess,
		keyPair:      keys,
		jitterBuffer: jitter.NewJitterBuffer(2, 20),
		role:         crypto.RoleHost,
		peerName:     "local-loopback",
		peerEndpoint: "127.0.0.1:internal",
		ctx:          ctx,
		cancel:       cancel,
	}

	s.connected.Store(true)
	s.setupLoopbackPipeline()
	return s, nil
}

// NewHostSession creates a hosting session that waits for a peer
func NewHostSession(port int) (*Session, string, error) {
	ctx, cancel := context.WithCancel(context.Background())

	audioEng, err := audio.NewEngine()
	if err != nil {
		cancel()
		return nil, "", fmt.Errorf("failed to init audio: %w", err)
	}

	opusCodec, err := codec.NewCodec()
	if err != nil {
		cancel()
		audioEng.Close()
		return nil, "", fmt.Errorf("failed to init opus codec: %w", err)
	}

	trans, err := network.NewTransport(port)
	if err != nil {
		cancel()
		audioEng.Close()
		return nil, "", fmt.Errorf("failed to bind UDP transport: %w", err)
	}

	keys, err := crypto.GenerateKeyPair()
	if err != nil {
		cancel()
		audioEng.Close()
		trans.Close()
		return nil, "", fmt.Errorf("failed to generate crypto keys: %w", err)
	}

	// Resolve public STUN endpoint
	publicAddr, err := network.DiscoverPublicEndpoint(nil, 3*time.Second)
	if err != nil {
		// Fallback to local address if offline
		publicAddr = trans.LocalAddr()
	}

	lanIP := GetLocalLANIP()
	lanAddr := &net.UDPAddr{IP: lanIP, Port: trans.LocalAddr().Port}

	token, err := EncodeDualToken(publicAddr, lanAddr, keys.PublicKeyBytes())
	if err != nil {
		cancel()
		audioEng.Close()
		trans.Close()
		return nil, "", fmt.Errorf("failed to encode room token: %w", err)
	}

	trans.SetLocalPublicKey(keys.PublicKeyBytes())

	s := &Session{
		mode:         ModeHost,
		audioEngine:  audioEng,
		codec:        opusCodec,
		keyPair:      keys,
		transport:    trans,
		jitterBuffer: jitter.NewJitterBuffer(2, 40),
		role:         crypto.RoleHost,
		peerName:     "Waiting for peer...",
		peerEndpoint: "listening on " + publicAddr.String(),
		roomToken:    token,
		ctx:          ctx,
		cancel:       cancel,
	}

	// Host establishes crypto session as soon as peer sends TypeHandshake with their public key
	trans.SetHandshakeHandlers(func(peerPubKey []byte, raddr *net.UDPAddr) {
		if len(peerPubKey) == 32 && s.getCrypto() == nil {
			cSess, err := crypto.NewCryptoSession(s.keyPair, peerPubKey, crypto.RoleHost)
			if err == nil {
				s.setCrypto(cSess, "Peer", raddr.String())
				s.connected.Store(true)
			}
		}
	}, nil)

	s.setupNetworkPipeline()
	return s, token, nil
}

// NewJoinSession creates a session that connects to a host via room token
func NewJoinSession(roomToken string) (*Session, error) {
	decoded, err := DecodeToken(roomToken)
	if err != nil {
		return nil, fmt.Errorf("invalid room token: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	audioEng, err := audio.NewEngine()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to init audio: %w", err)
	}

	opusCodec, err := codec.NewCodec()
	if err != nil {
		cancel()
		audioEng.Close()
		return nil, fmt.Errorf("failed to init opus codec: %w", err)
	}

	trans, err := network.NewTransport(0)
	if err != nil {
		cancel()
		audioEng.Close()
		return nil, fmt.Errorf("failed to bind UDP transport: %w", err)
	}

	keys, err := crypto.GenerateKeyPair()
	if err != nil {
		cancel()
		audioEng.Close()
		trans.Close()
		return nil, fmt.Errorf("failed to generate crypto keys: %w", err)
	}

	trans.SetLocalPublicKey(keys.PublicKeyBytes())

	cryptoSess, err := crypto.NewCryptoSession(keys, decoded.PublicKey, crypto.RolePeer)
	if err != nil {
		cancel()
		audioEng.Close()
		trans.Close()
		return nil, fmt.Errorf("failed to init crypto session: %w", err)
	}

	s := &Session{
		mode:         ModeJoin,
		audioEngine:  audioEng,
		codec:        opusCodec,
		crypto:       cryptoSess,
		keyPair:      keys,
		transport:    trans,
		jitterBuffer: jitter.NewJitterBuffer(2, 40),
		role:         crypto.RolePeer,
		peerName:     "Host Peer",
		peerEndpoint: decoded.PublicAddr.String(),
		roomToken:    roomToken,
		ctx:          ctx,
		cancel:       cancel,
	}

	s.setupNetworkPipeline()

	// Initiate UDP hole-punching against candidate endpoints (LAN + STUN) in background
	go func() {
		punchCtx, punchCancel := context.WithTimeout(s.ctx, 10*time.Second)
		defer punchCancel()

		punchErr := trans.PunchHoleCandidates(punchCtx, keys.PublicKeyBytes(), 10*time.Second, decoded.LANAddr, decoded.PublicAddr)
		if punchErr == nil {
			s.connected.Store(true)
			if trans.RemoteAddr() != nil {
				s.setPeerEndpoint(trans.RemoteAddr().String())
			}
		}
	}()

	return s, nil
}

// setupLoopbackPipeline connects mic -> encode -> encrypt -> decrypt -> decode -> speaker
func (s *Session) setupLoopbackPipeline() {
	var seqCounter uint64

	s.audioEngine.SetCaptureHandler(func(pcm []int16) {
		encoded, err := s.codec.Encode(pcm)
		if err != nil {
			return
		}

		seq := atomic.AddUint64(&seqCounter, 1)
		aad := []byte{protocol.MagicByte0, protocol.MagicByte1, protocol.Version1, byte(protocol.TypeAudio)}
		ciphertext := s.crypto.Encrypt(seq, encoded, aad)

		// Self-loopback: decrypt and push to jitter buffer
		decrypted, err := s.crypto.Decrypt(seq, ciphertext, aad)
		if err != nil {
			return
		}

		s.jitterBuffer.Push(seq, uint16(seq*20), decrypted, 10)
	})

	s.audioEngine.SetPlaybackProvider(func() []int16 {
		data, isPLC := s.jitterBuffer.Pop()
		if isPLC {
			pcm, err := s.codec.Decode(nil)
			if err == nil {
				return pcm
			}
			return nil
		}
		if len(data) == 0 {
			return nil
		}

		pcm, err := s.codec.Decode(data)
		if err != nil {
			return nil
		}
		return pcm
	})
}

// setupNetworkPipeline configures handlers for incoming and outgoing UDP packets
func (s *Session) setupNetworkPipeline() {
	// Capture Callback: Mic PCM -> Opus Encode -> ChaCha20 Encrypt -> SendPacket
	s.audioEngine.SetCaptureHandler(func(pcm []int16) {
		cr := s.getCrypto()
		if !s.connected.Load() || cr == nil {
			return
		}

		encoded, err := s.codec.Encode(pcm)
		if err != nil {
			return
		}

		seq := s.transport.NextSeq()
		aad := []byte{protocol.MagicByte0, protocol.MagicByte1, protocol.Version1, byte(protocol.TypeAudio)}
		ciphertext := cr.Encrypt(seq, encoded, aad)

		pkt := &protocol.Packet{
			Type:    protocol.TypeAudio,
			Seq:     seq,
			Payload: ciphertext,
		}
		_ = s.transport.SendPacket(pkt)
	})

	// Playback Provider: JitterBuffer Pop -> (Opus Decode or PLC) -> Speaker PCM
	s.audioEngine.SetPlaybackProvider(func() []int16 {
		data, isPLC := s.jitterBuffer.Pop()
		if isPLC {
			pcm, err := s.codec.Decode(nil)
			if err == nil {
				return pcm
			}
			return nil
		}
		if len(data) == 0 {
			return nil
		}

		pcm, err := s.codec.Decode(data)
		if err != nil {
			return nil
		}
		return pcm
	})

	// Network Incoming Handlers
	s.transport.SetHandlers(
		// onAudio
		func(packet *protocol.Packet) {
			cr := s.getCrypto()
			if cr == nil {
				return
			}
			aad := []byte{protocol.MagicByte0, protocol.MagicByte1, protocol.Version1, byte(protocol.TypeAudio)}
			decrypted, err := cr.Decrypt(packet.Seq, packet.Payload, aad)
			if err != nil {
				return // Dropped tampered or replayed packet
			}
			s.jitterBuffer.Push(packet.Seq, packet.Timestamp, decrypted, 10)
		},
		// onHangup
		func() {
			s.HangUp()
		},
		// onMute
		func(muted bool) {
			// Remote peer muted notification
		},
	)
}

// Start initiates the audio subsystem
func (s *Session) Start() error {
	if s.running.Swap(true) {
		return nil
	}
	return s.audioEngine.Start()
}

func (s *Session) getCrypto() *crypto.CryptoSession {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	return s.crypto
}

func (s *Session) setCrypto(cSess *crypto.CryptoSession, peerName, peerEndpoint string) {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	s.crypto = cSess
	if peerName != "" {
		s.peerName = peerName
	}
	if peerEndpoint != "" {
		s.peerEndpoint = peerEndpoint
	}
}

func (s *Session) setPeerEndpoint(endpoint string) {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	s.peerEndpoint = endpoint
}

// SessionState interface methods for the TUI:

func (s *Session) PeerName() string {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	return s.peerName
}

func (s *Session) PeerEndpoint() string {
	if s.transport != nil && s.transport.RemoteAddr() != nil {
		return s.transport.RemoteAddr().String()
	}
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	return s.peerEndpoint
}

func (s *Session) IsConnected() bool {
	if s.mode == ModeLoopback {
		return true
	}
	return s.connected.Load()
}

func (s *Session) IsMuted() bool {
	return s.audioEngine.IsMuted()
}

func (s *Session) Volume() int {
	return s.audioEngine.Volume()
}

func (s *Session) ToggleMute() bool {
	return s.audioEngine.ToggleMute()
}

func (s *Session) ChangeVolume(delta int) int {
	newVol := s.audioEngine.Volume() + delta
	s.audioEngine.SetVolume(newVol)
	return s.audioEngine.Volume()
}

func (s *Session) MicLevel() (float64, float64) {
	return s.audioEngine.MicLevel()
}

func (s *Session) SpeakerLevel() (float64, float64) {
	return s.audioEngine.SpeakerLevel()
}

func (s *Session) Telemetry() (time.Duration, float64, float64, int) {
	var rtt time.Duration
	if s.transport != nil {
		rtt = s.transport.RTT()
	}

	stats := s.jitterBuffer.Stats()
	var lossRate float64
	total := stats.PacketsReceived + stats.PacketsLost
	if total > 0 {
		lossRate = float64(stats.PacketsLost) / float64(total)
	}

	return rtt, lossRate, stats.CurrentJitterMs, stats.BufferDepthMs
}

func (s *Session) DebugInfo() string {
	if s.transport == nil {
		return fmt.Sprintf("Loopback | Buffer: %dms", s.jitterBuffer.Stats().BufferDepthMs)
	}
	return fmt.Sprintf("Tx: %d pkts | Rx: %d pkts | Local: %s | Remote: %s",
		s.transport.PacketsSent(),
		s.transport.PacketsReceived(),
		s.transport.LocalAddr().String(),
		s.PeerEndpoint(),
	)
}

func (s *Session) HangUp() {
	s.hangupMu.Lock()
	defer s.hangupMu.Unlock()

	s.cancel()
	s.audioEngine.Close()
	if s.transport != nil {
		s.transport.Close()
	}
	s.stateMu.Lock()
	if s.crypto != nil {
		s.crypto.Close()
	}
	s.stateMu.Unlock()
	if s.onHangup != nil {
		s.onHangup()
	}
}

