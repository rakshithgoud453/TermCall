package network

import (
	"bytes"
	"context"
	"sync"
	"testing"
	"time"

	"github.com/rakshithgoud453/termcall/pkg/protocol"
)

func TestSTUNDiscovery(t *testing.T) {
	addr, err := DiscoverPublicEndpoint(nil, 4*time.Second)
	if err != nil {
		t.Logf("STUN discovery failed (network might be offline or blocking UDP): %v", err)
		return
	}

	t.Logf("Successfully discovered public STUN endpoint: %s", addr.String())
	if addr.Port == 0 || addr.IP == nil {
		t.Fatalf("invalid STUN address received: %v", addr)
	}
}

func TestUDPTransportLocalPair(t *testing.T) {
	// Create two local transports on ephemeral ports
	t1, err := NewTransport(0)
	if err != nil {
		t.Fatalf("failed to create transport 1: %v", err)
	}
	defer t1.Close()

	t2, err := NewTransport(0)
	if err != nil {
		t.Fatalf("failed to create transport 2: %v", err)
	}
	defer t2.Close()

	var receivedData []byte
	var wg sync.WaitGroup
	wg.Add(1)

	t2.SetHandlers(func(p *protocol.Packet) {
		receivedData = p.Payload
		wg.Done()
	}, nil, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// t1 initiates hole punch to t2
	errCh := make(chan error, 1)
	go func() {
		errCh <- t1.PunchHole(ctx, t2.LocalAddr(), 2*time.Second)
	}()

	// Wait for punch to complete
	if err := <-errCh; err != nil {
		t.Fatalf("punch hole failed: %v", err)
	}

	if !t1.IsConnected() {
		t.Fatal("expected t1 to be marked connected")
	}

	// Send an audio packet from t1 to t2
	audioPayload := []byte("encrypted-sample-opus-payload")
	pkt := &protocol.Packet{
		Type:    protocol.TypeAudio,
		Seq:     t1.NextSeq(),
		Payload: audioPayload,
	}

	if err := t1.SendPacket(pkt); err != nil {
		t.Fatalf("failed to send packet: %v", err)
	}

	wg.Wait()

	if !bytes.Equal(receivedData, audioPayload) {
		t.Fatalf("payload mismatch: got %s, want %s", string(receivedData), string(audioPayload))
	}
}

func TestUDPTransportHangupNotification(t *testing.T) {
	t1, err := NewTransport(0)
	if err != nil {
		t.Fatalf("failed to create transport 1: %v", err)
	}
	defer t1.Close()

	t2, err := NewTransport(0)
	if err != nil {
		t.Fatalf("failed to create transport 2: %v", err)
	}
	defer t2.Close()

	hangupReceived := make(chan struct{}, 1)
	t2.SetHandlers(nil, func() {
		select {
		case hangupReceived <- struct{}{}:
		default:
		}
	}, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := t1.PunchHole(ctx, t2.LocalAddr(), 2*time.Second); err != nil {
		t.Fatalf("hole punch failed: %v", err)
	}

	if !t1.IsConnected() || !t2.IsConnected() {
		t.Fatal("expected both transports to be connected")
	}

	// Now close t1 - it should send hangup burst to t2
	if err := t1.Close(); err != nil {
		t.Fatalf("t1.Close failed: %v", err)
	}

	select {
	case <-hangupReceived:
		// Success!
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for t2 to receive hangup packet from t1")
	}

	if t2.IsConnected() {
		t.Fatal("expected t2.IsConnected() to be false after receiving hangup")
	}
}

