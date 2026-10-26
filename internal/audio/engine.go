package audio

import (
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/gen2brain/malgo"
	"github.com/rakshithgoud453/termcall/internal/codec"
)

const (
	SampleRate     = 48000
	Channels       = 1
	FrameSamples   = 960 // 20ms at 48kHz
	BytesPerSample = 2   // S16 (16-bit signed integer)
	FrameBytes     = FrameSamples * BytesPerSample
)

var (
	ErrEngineAlreadyRunning = errors.New("audio: engine is already running")
	ErrEngineNotRunning     = errors.New("audio: engine is not running")
)

// DeviceInfo represents an audio input or output device
type DeviceInfo struct {
	ID   string
	Name string
	Type string // "capture" or "playback"
}

// PlaybackProvider is a function that returns the next 960-sample PCM frame for playback
type PlaybackProvider func() []int16

// Engine handles microphone capture and speaker playback using miniaudio (malgo)
type Engine struct {
	ctx            *malgo.AllocatedContext
	captureDevice  *malgo.Device
	playbackDevice *malgo.Device

	running atomic.Bool
	muted   atomic.Bool

	// Volume multiplier in percentage (100 = 1.0x gain)
	volumePercent atomic.Int32

	// Real-time VU meter values (atomic bits)
	micDBNorm    atomic.Uint64 // packed (float32 dB, float32 norm)
	spkDBNorm    atomic.Uint64

	// Capture buffer for assembling 20ms frames
	capMu     sync.Mutex
	capBuffer []int16
	onCapture func([]int16)

	// Playback queue
	playMu       sync.Mutex
	playBuffer   []int16
	playProvider PlaybackProvider
}

// NewEngine initializes the audio subsystem
func NewEngine() (*Engine, error) {
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to init malgo context: %w", err)
	}

	e := &Engine{
		ctx:       ctx,
		capBuffer: make([]int16, 0, FrameSamples*2),
	}
	e.volumePercent.Store(100) // Default 100% volume
	return e, nil
}

// ListDevices returns all available capture and playback hardware devices
func ListDevices() ([]DeviceInfo, error) {
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return nil, err
	}
	defer ctx.Uninit()

	var result []DeviceInfo

	caps, err := ctx.Devices(malgo.Capture)
	if err == nil {
		for _, d := range caps {
			result = append(result, DeviceInfo{
				ID:   d.ID.String(),
				Name: d.Name(),
				Type: "capture",
			})
		}
	}

	plays, err := ctx.Devices(malgo.Playback)
	if err == nil {
		for _, d := range plays {
			result = append(result, DeviceInfo{
				ID:   d.ID.String(),
				Name: d.Name(),
				Type: "playback",
			})
		}
	}

	return result, nil
}

// SetCaptureHandler registers a callback for outgoing 20ms microphone frames
func (e *Engine) SetCaptureHandler(fn func([]int16)) {
	e.capMu.Lock()
	defer e.capMu.Unlock()
	e.onCapture = fn
}

// SetPlaybackProvider sets a custom frame provider for incoming audio
func (e *Engine) SetPlaybackProvider(provider PlaybackProvider) {
	e.playMu.Lock()
	defer e.playMu.Unlock()
	e.playProvider = provider
}

// EnqueuePlayback adds a decoded 20ms frame to the playback queue
func (e *Engine) EnqueuePlayback(pcm []int16) {
	e.playMu.Lock()
	defer e.playMu.Unlock()
	e.playBuffer = append(e.playBuffer, pcm...)
}

// Start begins hardware audio capture and playback
func (e *Engine) Start() error {
	if e.running.Load() {
		return ErrEngineAlreadyRunning
	}

	// 1. Configure Capture Device (Microphone)
	capConfig := malgo.DefaultDeviceConfig(malgo.Capture)
	capConfig.Capture.Format = malgo.FormatS16
	capConfig.Capture.Channels = Channels
	capConfig.SampleRate = SampleRate

	capCallbacks := malgo.DeviceCallbacks{
		Data: e.onCaptureData,
	}

	capDevice, err := malgo.InitDevice(e.ctx.Context, capConfig, capCallbacks)
	if err != nil {
		return fmt.Errorf("failed to init capture device: %w", err)
	}

	// 2. Configure Playback Device (Speakers / Headphones)
	playConfig := malgo.DefaultDeviceConfig(malgo.Playback)
	playConfig.Playback.Format = malgo.FormatS16
	playConfig.Playback.Channels = Channels
	playConfig.SampleRate = SampleRate

	playCallbacks := malgo.DeviceCallbacks{
		Data: e.onPlaybackData,
	}

	playDevice, err := malgo.InitDevice(e.ctx.Context, playConfig, playCallbacks)
	if err != nil {
		capDevice.Uninit()
		return fmt.Errorf("failed to init playback device: %w", err)
	}

	if err := capDevice.Start(); err != nil {
		capDevice.Uninit()
		playDevice.Uninit()
		return fmt.Errorf("failed to start capture device: %w", err)
	}

	if err := playDevice.Start(); err != nil {
		capDevice.Stop()
		capDevice.Uninit()
		playDevice.Uninit()
		return fmt.Errorf("failed to start playback device: %w", err)
	}

	e.captureDevice = capDevice
	e.playbackDevice = playDevice
	e.running.Store(true)

	return nil
}

// onCaptureData is invoked by malgo on incoming microphone frames
func (e *Engine) onCaptureData(pOutputSample, pInputSamples []byte, framecount uint32) {
	if len(pInputSamples) == 0 {
		return
	}

	numSamples := len(pInputSamples) / 2
	samples := make([]int16, numSamples)
	for i := 0; i < numSamples; i++ {
		samples[i] = int16(binary.LittleEndian.Uint16(pInputSamples[i*2 : (i+1)*2]))
	}

	e.capMu.Lock()
	e.capBuffer = append(e.capBuffer, samples...)

	// Drain complete 20ms frames (960 samples)
	for len(e.capBuffer) >= FrameSamples {
		frame := make([]int16, FrameSamples)
		copy(frame, e.capBuffer[:FrameSamples])
		e.capBuffer = e.capBuffer[FrameSamples:]

		// Calculate VU meter before muting (or muted shows 0)
		dB, norm := codec.CalculateRMSLevel(frame)
		e.storeLevels(&e.micDBNorm, dB, norm)

		if e.muted.Load() {
			// Zero out samples when muted
			for i := range frame {
				frame[i] = 0
			}
			e.storeLevels(&e.micDBNorm, -60.0, 0.0)
		}

		if e.onCapture != nil {
			go e.onCapture(frame)
		}
	}
	e.capMu.Unlock()
}

// onPlaybackData is invoked by malgo when the speaker needs audio frames
func (e *Engine) onPlaybackData(pOutputSample, pInputSamples []byte, framecount uint32) {
	neededSamples := int(framecount)
	outSamples := make([]int16, neededSamples)

	e.playMu.Lock()
	// Check if a custom playback provider is supplied (e.g. from jitter buffer)
	if e.playProvider != nil && len(e.playBuffer) < neededSamples {
		for len(e.playBuffer) < neededSamples {
			next := e.playProvider()
			if len(next) == 0 {
				break
			}
			e.playBuffer = append(e.playBuffer, next...)
		}
	}

	copied := copy(outSamples, e.playBuffer)
	e.playBuffer = e.playBuffer[copied:]
	e.playMu.Unlock()

	// Volume scaling
	volFactor := float64(e.volumePercent.Load()) / 100.0

	for i := 0; i < copied; i++ {
		scaled := float64(outSamples[i]) * volFactor
		if scaled > 32767.0 {
			scaled = 32767.0
		} else if scaled < -32768.0 {
			scaled = -32768.0
		}
		outSamples[i] = int16(scaled)
	}

	// Calculate speaker VU level
	if copied > 0 {
		dB, norm := codec.CalculateRMSLevel(outSamples[:copied])
		e.storeLevels(&e.spkDBNorm, dB, norm)
	} else {
		e.storeLevels(&e.spkDBNorm, -60.0, 0.0)
	}

	// Write out to output byte buffer in Little Endian S16
	for i := 0; i < neededSamples; i++ {
		binary.LittleEndian.PutUint16(pOutputSample[i*2:(i+1)*2], uint16(outSamples[i]))
	}
}

// Mute mutes the microphone
func (e *Engine) Mute() {
	e.muted.Store(true)
}

// Unmute unmutes the microphone
func (e *Engine) Unmute() {
	e.muted.Store(false)
}

// ToggleMute toggles mute state and returns the new state
func (e *Engine) ToggleMute() bool {
	for {
		current := e.muted.Load()
		if e.muted.CompareAndSwap(current, !current) {
			return !current
		}
	}
}

// IsMuted returns whether microphone is currently muted
func (e *Engine) IsMuted() bool {
	return e.muted.Load()
}

// SetVolume sets speaker volume in percent (0 - 200%)
func (e *Engine) SetVolume(percent int) {
	if percent < 0 {
		percent = 0
	} else if percent > 200 {
		percent = 200
	}
	e.volumePercent.Store(int32(percent))
}

// Volume returns current speaker volume percent
func (e *Engine) Volume() int {
	return int(e.volumePercent.Load())
}

// MicLevel returns the current microphone dB and normalized visual level [0.0, 1.0]
func (e *Engine) MicLevel() (float64, float64) {
	return e.loadLevels(&e.micDBNorm)
}

// SpeakerLevel returns the current speaker dB and normalized visual level [0.0, 1.0]
func (e *Engine) SpeakerLevel() (float64, float64) {
	return e.loadLevels(&e.spkDBNorm)
}

func (e *Engine) storeLevels(target *atomic.Uint64, dB, norm float64) {
	dbBits := uint32(int32(dB * 100))
	normBits := uint32(int32(norm * 1000))
	packed := (uint64(dbBits) << 32) | uint64(normBits)
	target.Store(packed)
}

func (e *Engine) loadLevels(target *atomic.Uint64) (float64, float64) {
	packed := target.Load()
	dbBits := int32(uint32(packed >> 32))
	normBits := int32(uint32(packed & 0xFFFFFFFF))
	return float64(dbBits) / 100.0, float64(normBits) / 1000.0
}

// Stop halts audio capture and playback
func (e *Engine) Stop() {
	if !e.running.Swap(false) {
		return
	}
	if e.captureDevice != nil {
		e.captureDevice.Stop()
		e.captureDevice.Uninit()
		e.captureDevice = nil
	}
	if e.playbackDevice != nil {
		e.playbackDevice.Stop()
		e.playbackDevice.Uninit()
		e.playbackDevice = nil
	}
}

// Close uninitializes the audio subsystem
func (e *Engine) Close() {
	e.Stop()
	if e.ctx != nil {
		e.ctx.Uninit()
		e.ctx.Free()
		e.ctx = nil
	}
}
