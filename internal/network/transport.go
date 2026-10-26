package network

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rakshithgoud453/termcall/pkg/protocol"
)

var (
	ErrNotConnected = errors.New("network: transport not connected to remote peer")
	ErrClosed       = errors.New("network: transport is closed")
)

type PacketHandler func(packet *protocol.Packet, raddr *net.UDPAddr)

// Transport manages UDP datagram transmission, hole-punching, and telemetry
type Transport struct {
	conn       *net.UDPConn
	localAddr  *net.UDPAddr
	remoteAddr *net.UDPAddr
	remoteMu   sync.RWMutex

	seqCounter atomic.Uint64

	// Handshake & State
	connected   atomic.Bool
	handshakeCh chan struct{}
	closed      atomic.Bool

	// Telemetry
	packetsSent     atomic.Uint64
	packetsReceived atomic.Uint64
	rttMs           atomic.Int64 // in milliseconds

	// Handshake callbacks & Keys
	localPubKey    []byte
	onHandshake    func(peerPubKey []byte, raddr *net.UDPAddr)
	onHandshakeAck func(peerPubKey []byte, raddr *net.UDPAddr)

	// Callbacks
	onAudio   func(packet *protocol.Packet)
	onHangup  func()
	onMute    func(muted bool)

	startTime time.Time
}

// NewTransport binds a local UDP socket on the specified port (0 for ephemeral)
func NewTransport(port int) (*Transport, error) {
	laddr := &net.UDPAddr{
		IP:   net.IPv4zero,
		Port: port,
	}

	conn, err := net.ListenUDP("udp4", laddr)
	if err != nil {
		return nil, fmt.Errorf("failed to bind UDP socket on port %d: %w", port, err)
	}

	actualAddr := conn.LocalAddr().(*net.UDPAddr)

	t := &Transport{
		conn:        conn,
		localAddr:   actualAddr,
		handshakeCh: make(chan struct{}, 1),
		startTime:   time.Now(),
	}
	t.rttMs.Store(0)

	// Start read loop
	go t.readLoop()

	return t, nil
}

// LocalAddr returns the bound local UDP address
func (t *Transport) LocalAddr() *net.UDPAddr {
	return t.localAddr
}

// RemoteAddr returns the current remote peer address
func (t *Transport) RemoteAddr() *net.UDPAddr {
	t.remoteMu.RLock()
	defer t.remoteMu.RUnlock()
	return t.remoteAddr
}

// normalizeAddr ensures unspecified 0.0.0.0 addresses are resolved to 127.0.0.1 for local routing
func normalizeAddr(addr *net.UDPAddr) *net.UDPAddr {
	if addr == nil {
		return nil
	}
	if addr.IP == nil || addr.IP.IsUnspecified() {
		return &net.UDPAddr{
			IP:   net.IPv4(127, 0, 0, 1),
			Port: addr.Port,
			Zone: addr.Zone,
		}
	}
	return addr
}

// SetRemoteAddr configures the remote peer address
func (t *Transport) SetRemoteAddr(addr *net.UDPAddr) {
	t.remoteMu.Lock()
	defer t.remoteMu.Unlock()
	t.remoteAddr = normalizeAddr(addr)
}

// SetLocalPublicKey configures the local public key for handshake packet payloads
func (t *Transport) SetLocalPublicKey(pubKey []byte) {
	t.localPubKey = pubKey
}

// SetHandshakeHandlers sets callbacks for peer key exchange during P2P punch
func (t *Transport) SetHandshakeHandlers(onHandshake func([]byte, *net.UDPAddr), onHandshakeAck func([]byte, *net.UDPAddr)) {
	t.onHandshake = onHandshake
	t.onHandshakeAck = onHandshakeAck
}

// SetHandlers sets callbacks for incoming packets
func (t *Transport) SetHandlers(onAudio func(*protocol.Packet), onHangup func(), onMute func(bool)) {
	t.onAudio = onAudio
	t.onHangup = onHangup
	t.onMute = onMute
}

// NextSeq returns the next monotonic sequence number
func (t *Transport) NextSeq() uint64 {
	return t.seqCounter.Add(1)
}

// SendPacket sends a protocol packet to the configured remote peer
func (t *Transport) SendPacket(packet *protocol.Packet) error {
	if t.closed.Load() {
		return ErrClosed
	}

	target := t.RemoteAddr()
	if target == nil {
		return ErrNotConnected
	}

	// Stamp timestamp in relative milliseconds
	elapsedMs := uint16(time.Since(t.startTime).Milliseconds() & 0xFFFF)
	if packet.Timestamp == 0 {
		packet.Timestamp = elapsedMs
	}

	data := packet.Encode()
	_, err := t.conn.WriteToUDP(data, target)
	if err != nil {
		return err
	}

	t.packetsSent.Add(1)
	return nil
}

// SendTo sends a protocol packet to an explicit address (used during hole punching)
func (t *Transport) SendTo(packet *protocol.Packet, target *net.UDPAddr) error {
	if t.closed.Load() {
		return ErrClosed
	}
	target = normalizeAddr(target)
	data := packet.Encode()
	_, err := t.conn.WriteToUDP(data, target)
	if err == nil {
		t.packetsSent.Add(1)
	}
	return err
}

// PunchHole sends bursts of handshake packets to the remote peer to punch through NATs
func (t *Transport) PunchHole(ctx context.Context, target *net.UDPAddr, timeout time.Duration) error {
	return t.PunchHoleCandidates(ctx, t.localPubKey, timeout, target)
}

// PunchHoleCandidates sends handshake bursts to candidate endpoints (e.g. STUN public + LAN private)
// until any candidate responds, dynamically selecting the fastest working path.
func (t *Transport) PunchHoleCandidates(ctx context.Context, localPubKey []byte, timeout time.Duration, targets ...*net.UDPAddr) error {
	if len(localPubKey) > 0 {
		t.localPubKey = localPubKey
	}

	var validTargets []*net.UDPAddr
	seen := make(map[string]bool)
	for _, target := range targets {
		if target == nil {
			continue
		}
		norm := normalizeAddr(target)
		key := norm.String()
		if !seen[key] {
			seen[key] = true
			validTargets = append(validTargets, norm)
		}
	}

	if len(validTargets) == 0 {
		return errors.New("no valid candidate targets provided")
	}

	ticker := time.NewTicker(80 * time.Millisecond)
	defer ticker.Stop()

	deadline := time.After(timeout)

	sendBurst := func() {
		for _, target := range validTargets {
			pkt := &protocol.Packet{
				Type:    protocol.TypeHandshake,
				Seq:     t.NextSeq(),
				Payload: t.localPubKey,
			}
			_ = t.SendTo(pkt, target)
		}
	}

	// Send initial burst
	for i := 0; i < 3; i++ {
		sendBurst()
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline:
			return errors.New("NAT hole-punching timed out")
		case <-t.handshakeCh:
			t.connected.Store(true)
			go t.pingLoop(ctx)
			return nil
		case <-ticker.C:
			sendBurst()
		}
	}
}

// MarkConnected manually marks the connection established (e.g. for local loopback or tested connections)
func (t *Transport) MarkConnected(remote *net.UDPAddr) {
	t.SetRemoteAddr(remote)
	t.connected.Store(true)
}

// IsConnected returns whether direct P2P link is established
func (t *Transport) IsConnected() bool {
	return t.connected.Load()
}

// RTT returns the current round-trip latency estimate
func (t *Transport) RTT() time.Duration {
	ms := t.rttMs.Load()
	return time.Duration(ms) * time.Millisecond
}

// PacketsSent returns count of transmitted datagrams
func (t *Transport) PacketsSent() uint64 {
	return t.packetsSent.Load()
}

// PacketsReceived returns count of received datagrams
func (t *Transport) PacketsReceived() uint64 {
	return t.packetsReceived.Load()
}

// readLoop processes incoming UDP datagrams
func (t *Transport) readLoop() {
	buf := make([]byte, 2048)

	for {
		n, raddr, err := t.conn.ReadFromUDP(buf)
		if err != nil {
			if t.closed.Load() {
				return
			}
			continue
		}

		packet, err := protocol.Decode(buf[:n])
		if err != nil {
			// Ignore non-protocol packets / random scanner traffic
			continue
		}

		t.packetsReceived.Add(1)

		switch packet.Type {
		case protocol.TypeHandshake:
			// If we haven't locked remote address, lock it to sender
			if t.RemoteAddr() == nil {
				t.SetRemoteAddr(raddr)
			}
			// Respond with HandshakeAck containing our public key
			ack := &protocol.Packet{
				Type:    protocol.TypeHandshakeAck,
				Seq:     t.NextSeq(),
				Payload: t.localPubKey,
			}
			_ = t.SendTo(ack, raddr)

			if t.onHandshake != nil {
				t.onHandshake(packet.Payload, raddr)
			}

			select {
			case t.handshakeCh <- struct{}{}:
			default:
			}

		case protocol.TypeHandshakeAck:
			if t.RemoteAddr() == nil {
				t.SetRemoteAddr(raddr)
			}
			if t.onHandshakeAck != nil {
				t.onHandshakeAck(packet.Payload, raddr)
			}
			select {
			case t.handshakeCh <- struct{}{}:
			default:
			}

		case protocol.TypePing:
			// Echo back as Pong with same timestamp
			pong := &protocol.Packet{
				Type:      protocol.TypePong,
				Seq:       t.NextSeq(),
				Timestamp: packet.Timestamp,
			}
			_ = t.SendTo(pong, raddr)

		case protocol.TypePong:
			// Compute RTT
			nowMs := uint16(time.Since(t.startTime).Milliseconds() & 0xFFFF)
			if nowMs >= packet.Timestamp {
				sampleRTT := int64(nowMs - packet.Timestamp)
				oldRTT := t.rttMs.Load()
				if oldRTT == 0 {
					t.rttMs.Store(sampleRTT)
				} else {
					// Exponential moving average: 0.8 * old + 0.2 * new
					newRTT := int64(float64(oldRTT)*0.8 + float64(sampleRTT)*0.2)
					t.rttMs.Store(newRTT)
				}
			}

		case protocol.TypeAudio:
			if t.onAudio != nil {
				t.onAudio(packet)
			}

		case protocol.TypeMute:
			if t.onMute != nil && len(packet.Payload) > 0 {
				t.onMute(packet.Payload[0] == 1)
			}

		case protocol.TypeHangup:
			if t.onHangup != nil {
				t.onHangup()
			}
		}
	}
}

// pingLoop sends periodic keepalive probes to measure RTT latency
func (t *Transport) pingLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if t.closed.Load() || !t.connected.Load() {
				return
			}
			ping := &protocol.Packet{
				Type: protocol.TypePing,
				Seq:  t.NextSeq(),
			}
			_ = t.SendPacket(ping)
		}
	}
}

// Close closes the UDP socket
func (t *Transport) Close() error {
	if !t.closed.Swap(true) {
		// Send hangup notice to peer if connected
		if t.connected.Load() && t.RemoteAddr() != nil {
			hangup := &protocol.Packet{
				Type: protocol.TypeHangup,
				Seq:  t.NextSeq(),
			}
			_ = t.SendPacket(hangup)
		}
		return t.conn.Close()
	}
	return nil
}
