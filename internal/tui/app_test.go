package tui

import (
	"strings"
	"testing"
	"time"
)

type mockSession struct{}

func (m *mockSession) PeerName() string                                                 { return "mock-peer" }
func (m *mockSession) PeerEndpoint() string                                             { return "127.0.0.1:51820" }
func (m *mockSession) IsConnected() bool                                                { return true }
func (m *mockSession) IsMuted() bool                                                    { return false }
func (m *mockSession) Volume() int                                                      { return 100 }
func (m *mockSession) ToggleMute() bool                                                 { return false }
func (m *mockSession) ChangeVolume(delta int) int                                       { return 100 + delta }
func (m *mockSession) MicLevel() (float64, float64)                                     { return -18.0, 0.6 }
func (m *mockSession) SpeakerLevel() (float64, float64)                                 { return -12.0, 0.7 }
func (m *mockSession) Telemetry() (time.Duration, float64, float64, int)                { return 24 * time.Millisecond, 0.0, 1.2, 40 }
func (m *mockSession) DebugInfo() string                                                { return "Sent: 100, Recv: 100" }
func (m *mockSession) HangUp()                                                          {}

func TestTUIModelRender(t *testing.T) {
	mock := &mockSession{}
	model := NewModel(mock)

	view := model.View()
	if !strings.Contains(view, "TERMCALL") {
		t.Errorf("expected view to contain 'TERMCALL', got:\n%s", view)
	}
	if !strings.Contains(view, "mock-peer") {
		t.Errorf("expected view to contain 'mock-peer'")
	}
	if !strings.Contains(view, "X25519") {
		t.Errorf("expected view to contain crypto details")
	}
}

func TestRenderMeter(t *testing.T) {
	meter0 := renderMeter(0.0, 10, false)
	if !strings.Contains(meter0, "□□□□□□□□□□") {
		t.Errorf("expected all empty boxes, got: %s", meter0)
	}

	meterHalf := renderMeter(0.5, 10, false)
	if !strings.Contains(meterHalf, "■■■■■") {
		t.Errorf("expected 5 filled boxes, got: %s", meterHalf)
	}

	meterMuted := renderMeter(0.8, 10, true)
	if strings.Contains(meterMuted, "■") {
		t.Errorf("expected no filled boxes when muted, got: %s", meterMuted)
	}
}
