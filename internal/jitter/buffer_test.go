package jitter

import (
	"bytes"
	"testing"
)

func TestJitterBufferInOrder(t *testing.T) {
	jb := NewJitterBuffer(2, 20)

	// Push 3 frames
	jb.Push(1, 100, []byte("frame-1"), 10)
	jb.Push(2, 120, []byte("frame-2"), 12)
	jb.Push(3, 140, []byte("frame-3"), 11)

	// Pop 1
	data, plc := jb.Pop()
	if plc || !bytes.Equal(data, []byte("frame-1")) {
		t.Fatalf("expected frame-1, got %s (plc=%v)", string(data), plc)
	}

	// Pop 2
	data, plc = jb.Pop()
	if plc || !bytes.Equal(data, []byte("frame-2")) {
		t.Fatalf("expected frame-2, got %s (plc=%v)", string(data), plc)
	}

	// Pop 3
	data, plc = jb.Pop()
	if plc || !bytes.Equal(data, []byte("frame-3")) {
		t.Fatalf("expected frame-3, got %s (plc=%v)", string(data), plc)
	}
}

func TestJitterBufferReorderAndLoss(t *testing.T) {
	jb := NewJitterBuffer(2, 20)

	// Send out of order: 1, 3, 2 (frame 2 delayed)
	jb.Push(1, 100, []byte("frame-1"), 10)
	jb.Push(3, 140, []byte("frame-3"), 15)
	jb.Push(2, 120, []byte("frame-2"), 12)

	// Should play in order: 1, 2, 3
	d1, plc1 := jb.Pop()
	if plc1 || !bytes.Equal(d1, []byte("frame-1")) {
		t.Fatalf("expected frame-1, got %s", string(d1))
	}

	d2, plc2 := jb.Pop()
	if plc2 || !bytes.Equal(d2, []byte("frame-2")) {
		t.Fatalf("expected frame-2, got %s", string(d2))
	}

	d3, plc3 := jb.Pop()
	if plc3 || !bytes.Equal(d3, []byte("frame-3")) {
		t.Fatalf("expected frame-3, got %s", string(d3))
	}

	// Now send 5 (frame 4 lost completely!)
	jb.Push(5, 180, []byte("frame-5"), 14)

	// Next expected was 4. Since 4 is missing and 5 has arrived, Pop() should return plc=true
	d4, plc4 := jb.Pop()
	if !plc4 || d4 != nil {
		t.Fatalf("expected PLC trigger for missing frame-4, got %v (plc=%v)", d4, plc4)
	}

	// Next is frame 5
	d5, plc5 := jb.Pop()
	if plc5 || !bytes.Equal(d5, []byte("frame-5")) {
		t.Fatalf("expected frame-5, got %s", string(d5))
	}

	stats := jb.Stats()
	if stats.PacketsLost != 1 {
		t.Errorf("expected 1 lost packet, got %d", stats.PacketsLost)
	}
	if stats.PacketsConcealed != 1 {
		t.Errorf("expected 1 concealed packet, got %d", stats.PacketsConcealed)
	}
}
