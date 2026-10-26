package audio

import (
	"testing"
	"time"
)

func TestListDevices(t *testing.T) {
	devices, err := ListDevices()
	if err != nil {
		t.Fatalf("failed to list devices: %v", err)
	}

	t.Logf("Discovered %d audio devices:", len(devices))
	for _, d := range devices {
		t.Logf(" - [%s] %s (id: %s)", d.Type, d.Name, d.ID)
	}

	if len(devices) == 0 {
		t.Log("Warning: no audio devices detected on this system")
	}
}

func TestEngineLifecycleAndControls(t *testing.T) {
	engine, err := NewEngine()
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}
	defer engine.Close()

	// Verify volume controls
	if engine.Volume() != 100 {
		t.Errorf("expected default volume 100, got %d", engine.Volume())
	}

	engine.SetVolume(120)
	if engine.Volume() != 120 {
		t.Errorf("expected volume 120, got %d", engine.Volume())
	}

	// Verify mute controls
	if engine.IsMuted() {
		t.Error("expected initially unmuted")
	}

	engine.Mute()
	if !engine.IsMuted() {
		t.Error("expected muted after Mute()")
	}

	engine.Unmute()
	if engine.IsMuted() {
		t.Error("expected unmuted after Unmute()")
	}

	toggled := engine.ToggleMute()
	if !toggled || !engine.IsMuted() {
		t.Error("expected muted after ToggleMute()")
	}

	// Verify VU level packing/unpacking
	dB, norm := engine.MicLevel()
	if dB > 0.0 || norm < 0.0 {
		t.Errorf("unexpected initial mic level: %f dB, %f norm", dB, norm)
	}

	// Start engine briefly if capture device is present
	err = engine.Start()
	if err != nil {
		t.Logf("Start() returned error (expected in headless or test env without mic permission): %v", err)
		return
	}

	time.Sleep(100 * time.Millisecond)
	engine.Stop()
}
