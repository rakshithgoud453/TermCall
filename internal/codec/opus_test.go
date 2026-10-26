package codec

import (
	"math"
	"testing"
)

func TestOpusEncodeDecodeAndPLC(t *testing.T) {
	c, err := NewCodec()
	if err != nil {
		t.Fatalf("failed to initialize codec: %v", err)
	}

	// Generate a 440 Hz sine wave PCM tone (960 samples = 20ms at 48kHz)
	pcm := make([]int16, FrameSize)
	for i := range pcm {
		// 440 Hz tone
		angle := 2.0 * math.Pi * 440.0 * float64(i) / float64(SampleRate)
		pcm[i] = int16(math.Sin(angle) * 16384.0) // 50% amplitude
	}

	dB, norm := CalculateRMSLevel(pcm)
	if dB < -25.0 || dB > -5.0 {
		t.Errorf("unexpected dB level for 50%% sine wave: %f dB", dB)
	}
	if norm <= 0.0 || norm > 1.0 {
		t.Errorf("unexpected normalized level: %f", norm)
	}

	// Encode to Opus
	encoded, err := c.Encode(pcm)
	if err != nil {
		t.Fatalf("failed to encode pcm: %v", err)
	}
	if len(encoded) == 0 {
		t.Fatal("encoded opus packet is empty")
	}

	// Decode back to PCM
	decoded, err := c.Decode(encoded)
	if err != nil {
		t.Fatalf("failed to decode opus: %v", err)
	}
	if len(decoded) != FrameSize {
		t.Fatalf("decoded pcm size mismatch: got %d, want %d", len(decoded), FrameSize)
	}

	// Test Packet Loss Concealment (PLC) by passing nil/empty
	plcDecoded, err := c.Decode(nil)
	if err != nil {
		t.Fatalf("failed to perform PLC decode: %v", err)
	}
	if len(plcDecoded) != FrameSize {
		t.Fatalf("PLC decoded pcm size mismatch: got %d, want %d", len(plcDecoded), FrameSize)
	}

	// Test dynamic bitrate adjustment
	if err := c.SetBitrate(16000); err != nil {
		t.Fatalf("failed to set 16kbps bitrate: %v", err)
	}
	if c.Bitrate() != 16000 {
		t.Errorf("bitrate mismatch: got %d, want 16000", c.Bitrate())
	}
	if err := c.SetBitrate(32000); err != nil {
		t.Fatalf("failed to set 32kbps bitrate: %v", err)
	}
	if c.Bitrate() != 32000 {
		t.Errorf("bitrate mismatch: got %d, want 32000", c.Bitrate())
	}

	// Test dynamic FEC packet loss percentage
	if err := c.SetPacketLossPerc(15); err != nil {
		t.Fatalf("failed to set FEC packet loss: %v", err)
	}
	if c.PacketLossPerc() != 15 {
		t.Errorf("packet loss perc mismatch: got %d, want 15", c.PacketLossPerc())
	}
}
