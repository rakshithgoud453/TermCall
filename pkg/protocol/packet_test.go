package protocol

import (
	"bytes"
	"testing"
)

func TestPacketEncodeDecode(t *testing.T) {
	original := &Packet{
		Version:   Version1,
		Type:      TypeAudio,
		Seq:       1048576,
		Timestamp: 45231,
		Payload:   []byte("test-opus-audio-payload-data-here-12345"),
	}

	encoded := original.Encode()
	if len(encoded) != HeaderSize+len(original.Payload) {
		t.Fatalf("expected encoded len %d, got %d", HeaderSize+len(original.Payload), len(encoded))
	}

	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}

	if decoded.Version != original.Version {
		t.Errorf("version mismatch: got %d, want %d", decoded.Version, original.Version)
	}
	if decoded.Type != original.Type {
		t.Errorf("type mismatch: got %d, want %d", decoded.Type, original.Type)
	}
	if decoded.Seq != original.Seq {
		t.Errorf("seq mismatch: got %d, want %d", decoded.Seq, original.Seq)
	}
	if decoded.Timestamp != original.Timestamp {
		t.Errorf("timestamp mismatch: got %d, want %d", decoded.Timestamp, original.Timestamp)
	}
	if !bytes.Equal(decoded.Payload, original.Payload) {
		t.Errorf("payload mismatch: got %v, want %v", decoded.Payload, original.Payload)
	}
}

func TestPacketDecodeErrors(t *testing.T) {
	_, err := Decode([]byte{0x01, 0x02})
	if err != ErrPacketTooShort {
		t.Errorf("expected ErrPacketTooShort, got %v", err)
	}

	badMagic := make([]byte, HeaderSize)
	badMagic[0] = 0xAA
	badMagic[1] = 0xBB
	_, err = Decode(badMagic)
	if err != ErrInvalidMagic {
		t.Errorf("expected ErrInvalidMagic, got %v", err)
	}
}
