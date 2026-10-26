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
	var receivedPayload []byte
	var wg sync.WaitGroup
	wg.Add(1)

	// Temporarily hook join transport to capture raw audio packet
	joinSess.transport.SetHandlers(func(packet *protocol.Packet) {
		aad := []byte{protocol.MagicByte0, protocol.MagicByte1, protocol.Version1, byte(protocol.TypeAudio)}
		decrypted, err := joinSess.crypto.Decrypt(packet.Seq, packet.Payload, aad)
		if err == nil {
			receivedPayload = decrypted
			wg.Done()
		}
	}, nil, nil)

	// Host sends encrypted audio frame
	sampleOpus := []byte("real-audio-opus-test-data-960-samples")
	seq := hostSess.transport.NextSeq()
	aad := []byte{protocol.MagicByte0, protocol.MagicByte1, protocol.Version1, byte(protocol.TypeAudio)}
	ciphertext := hostSess.crypto.Encrypt(seq, sampleOpus, aad)

	pkt := &protocol.Packet{
		Type:    protocol.TypeAudio,
		Seq:     seq,
		Payload: ciphertext,
	}

	if err := hostSess.transport.SendPacket(pkt); err != nil {
		t.Fatalf("host failed to send packet: %v", err)
	}

	wg.Wait()

	if !bytes.Equal(receivedPayload, sampleOpus) {
		t.Fatalf("decrypted audio payload mismatch: got %s, want %s", string(receivedPayload), string(sampleOpus))
	}
	t.Logf("✔ Encrypted audio frame successfully transmitted, decrypted, and authenticated!")
}
