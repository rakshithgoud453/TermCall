package session

import (
	"bytes"
	"sync"
	"testing"
	"time"

	"github.com/rakshithgoud453/termcall/pkg/protocol"
)

func TestHostJoinSessionPair(t *testing.T) {
	// 1. Dev A spins up Host session on random ephemeral port
	hostSess, token, err := NewHostSession(0)
	if err != nil {
		t.Fatalf("failed to create host session: %v", err)
	}
	defer hostSess.HangUp()

	t.Logf("Dev A (Host) generated Room Token: %s", token)

	// 2. Dev B joins using the Room Token
	joinSess, err := NewJoinSession(token)
	if err != nil {
		t.Fatalf("failed to create join session: %v", err)
	}
	defer joinSess.HangUp()

	// 3. Wait for direct P2P hole-punching and X25519 handshake to complete
	deadline := time.After(3 * time.Second)
	connected := false
	for !connected {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for Host and Joiner P2P link to connect")
		default:
			if hostSess.IsConnected() && joinSess.IsConnected() {
				connected = true
				break
			}
			time.Sleep(20 * time.Millisecond)
		}
	}

	t.Logf("✔ Direct P2P link established!")
	t.Logf("  Host connected to: %s", hostSess.PeerEndpoint())
	t.Logf("  Joiner connected to: %s", joinSess.PeerEndpoint())

	// 4. Test encrypted audio transmission from Host to Joiner
	var (
		receivedMu      sync.Mutex
		receivedPayload []byte
		receivedOnce    sync.Once
	)
	var wg sync.WaitGroup
	wg.Add(1)

	// Temporarily hook join transport to capture raw audio packet
	joinSess.transport.SetHandlers(func(packet *protocol.Packet) {
		aad := []byte{protocol.MagicByte0, protocol.MagicByte1, protocol.Version1, byte(protocol.TypeAudio)}
		joinCrypto := joinSess.getCrypto()
		if joinCrypto == nil {
			return
		}
		decrypted, err := joinCrypto.Decrypt(packet.Seq, packet.Payload, aad)
		if err == nil {
			receivedOnce.Do(func() {
				receivedMu.Lock()
				receivedPayload = decrypted
				receivedMu.Unlock()
				wg.Done()
			})
		}
	}, nil, nil)

	// Host sends encrypted audio frame
	sampleOpus := []byte("real-audio-opus-test-data-960-samples")
	seq := hostSess.transport.NextSeq()
	aad := []byte{protocol.MagicByte0, protocol.MagicByte1, protocol.Version1, byte(protocol.TypeAudio)}
	hostCrypto := hostSess.getCrypto()
	if hostCrypto == nil {
		t.Fatalf("host crypto session is nil")
	}
	ciphertext := hostCrypto.Encrypt(seq, sampleOpus, aad)

	pkt := &protocol.Packet{
		Type:    protocol.TypeAudio,
		Seq:     seq,
		Payload: ciphertext,
	}

	if err := hostSess.transport.SendPacket(pkt); err != nil {
		t.Fatalf("host failed to send packet: %v", err)
	}

	wg.Wait()

	receivedMu.Lock()
	got := receivedPayload
	receivedMu.Unlock()

	if !bytes.Equal(got, sampleOpus) {
		t.Fatalf("decrypted audio payload mismatch: got %s, want %s", string(got), string(sampleOpus))
	}
	t.Logf("✔ Encrypted audio frame successfully transmitted, decrypted, and authenticated!")
}
