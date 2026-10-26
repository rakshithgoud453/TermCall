package codec

import (
	"errors"
	"fmt"
	"math"

	"gopkg.in/hraban/opus.v2"
)

const (
	SampleRate     = 48000
	Channels       = 1                // Mono for VoIP voice calls
	FrameDurationMs = 20              // 20ms audio frames
	FrameSize      = SampleRate * FrameDurationMs / 1000 // 960 samples per 20ms frame
	MaxPacketSize  = 1275             // Standard max Opus packet size
	DefaultBitrate = 24000            // 24 kbps
)

var (
	ErrInvalidPCMSize = errors.New("codec: PCM buffer size does not match expected frame size")
)

// Codec handles Opus audio encoding and decoding with PLC support
type Codec struct {
	encoder *opus.Encoder
	decoder *opus.Decoder
}

// NewCodec initializes an Opus VoIP encoder and decoder at 48kHz Mono
func NewCodec() (*Codec, error) {
	enc, err := opus.NewEncoder(SampleRate, Channels, opus.AppVoIP)
	if err != nil {
		return nil, fmt.Errorf("failed to create opus encoder: %w", err)
	}

	if err := enc.SetBitrate(DefaultBitrate); err != nil {
		return nil, fmt.Errorf("failed to set encoder bitrate: %w", err)
	}

	if err := enc.SetInBandFEC(true); err != nil {
		return nil, fmt.Errorf("failed to set encoder in-band FEC: %w", err)
	}

	dec, err := opus.NewDecoder(SampleRate, Channels)
	if err != nil {
		return nil, fmt.Errorf("failed to create opus decoder: %w", err)
	}

	return &Codec{
		encoder: enc,
		decoder: dec,
	}, nil
}

// Encode encodes raw 16-bit PCM samples into an Opus packet
func (c *Codec) Encode(pcm []int16) ([]byte, error) {
	if len(pcm) != FrameSize {
		return nil, fmt.Errorf("%w: expected %d, got %d", ErrInvalidPCMSize, FrameSize, len(pcm))
	}

	out := make([]byte, MaxPacketSize)
	n, err := c.encoder.Encode(pcm, out)
	if err != nil {
		return nil, fmt.Errorf("opus encode failed: %w", err)
	}

	return out[:n], nil
}

// Decode decodes an Opus packet into raw 16-bit PCM samples.
// If opusData is nil or empty, Opus Packet Loss Concealment (PLC) is triggered.
func (c *Codec) Decode(opusData []byte) ([]int16, error) {
	pcm := make([]int16, FrameSize)
	if len(opusData) == 0 {
		// Packet Loss Concealment (PLC)
		if err := c.decoder.DecodePLC(pcm); err != nil {
			return nil, fmt.Errorf("opus PLC decode failed: %w", err)
		}
		return pcm, nil
	}

	n, err := c.decoder.Decode(opusData, pcm)
	if err != nil {
		return nil, fmt.Errorf("opus decode failed: %w", err)
	}

	return pcm[:n], nil
}

// CalculateRMSLevel computes the Root Mean Square (RMS) decibel level of a PCM frame.
// Returns dB in the range [-60 dB, 0 dB] and a normalized visual level [0.0, 1.0].
func CalculateRMSLevel(pcm []int16) (float64, float64) {
	if len(pcm) == 0 {
		return -60.0, 0.0
	}

	var sumSq float64
	for _, sample := range pcm {
		f := float64(sample)
		sumSq += f * f
	}

	rms := math.Sqrt(sumSq / float64(len(pcm)))
	if rms < 1.0 {
		return -60.0, 0.0
	}

	// 32767 is max amplitude for 16-bit signed PCM
	dB := 20.0 * math.Log10(rms/32767.0)
	if dB < -60.0 {
		dB = -60.0
	} else if dB > 0.0 {
		dB = 0.0
	}

	// Normalize dB between -50 dB (quiet) and -6 dB (loud) to [0.0, 1.0]
	minDB := -50.0
	maxDB := -6.0
	normalized := (dB - minDB) / (maxDB - minDB)
	if normalized < 0.0 {
		normalized = 0.0
	} else if normalized > 1.0 {
		normalized = 1.0
	}

	return dB, normalized
}
