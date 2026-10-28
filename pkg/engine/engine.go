package engine

import (
	"time"

	"github.com/rakshithgoud453/termcall/internal/session"
)

// CallSession defines the interactive interface for controlling a voice call session.
type CallSession interface {
	Start() error
	IsConnected() bool
	IsMuted() bool
	Volume() int
	ToggleMute() bool
	ChangeVolume(delta int) int
	MicLevel() (float64, float64)      // dB, norm [0.0, 1.0]
	SpeakerLevel() (float64, float64)  // dB, norm [0.0, 1.0]
	Telemetry() (rtt time.Duration, lossRate float64, jitterMs float64, bufferMs int)
	PeerName() string
	PeerEndpoint() string
	SetOnHangup(fn func())
	HangUp()
}

// Version of the underlying TermCall engine
const Version = "0.2.1"

// NewHost creates a host session and returns the session, room token, and any error.
func NewHost(port int) (CallSession, string, error) {
	sess, token, err := session.NewHostSession(port)
	if err != nil {
		return nil, "", err
	}
	return sess, token, nil
}

// NewJoin creates a join session connected to the given room token.
func NewJoin(token string) (CallSession, error) {
	sess, err := session.NewJoinSession(token)
	if err != nil {
		return nil, err
	}
	return sess, nil
}
