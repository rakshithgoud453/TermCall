package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SessionState provides real-time information from the active audio/crypto/network session
type SessionState interface {
	PeerName() string
	PeerEndpoint() string
	IsConnected() bool
	IsMuted() bool
	Volume() int
	ToggleMute() bool
	ChangeVolume(delta int) int
	MicLevel() (float64, float64)      // dB, norm [0.0, 1.0]
	SpeakerLevel() (float64, float64)  // dB, norm [0.0, 1.0]
	Telemetry() (rtt time.Duration, lossRate float64, jitterMs float64, bufferMs int)
	DebugInfo() string
	HangUp()
}

type tickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(50*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// Version is dynamically displayed in the TUI header
var Version = "0.2.1"

// Model is the Bubble Tea TUI state
type Model struct {
	state       SessionState
	showDebug   bool
	quitting    bool
	width       int
	height      int
	statusMsg   string
	statusTime  time.Time
}

// NewModel creates a new TUI model bound to a session state
func NewModel(state SessionState) Model {
	return Model{
		state:  state,
		width:  80,
		height: 24,
	}
}

func (m Model) Init() tea.Cmd {
	return tickCmd()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tickMsg:
		if m.quitting {
			return m, tea.Quit
		}
		return m, tickCmd()

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			m.quitting = true
			if m.state != nil {
				m.state.HangUp()
			}
			return m, tea.Quit

		case " ":
			if m.state != nil {
				muted := m.state.ToggleMute()
				if muted {
					m.setStatus("Microphone MUTED")
				} else {
					m.setStatus("Microphone LIVE")
				}
			}
			return m, nil

		case "up", "+", "=":
			if m.state != nil {
				vol := m.state.ChangeVolume(5)
				m.setStatus(fmt.Sprintf("Volume: %d%%", vol))
			}
			return m, nil

		case "down", "-", "_":
			if m.state != nil {
				vol := m.state.ChangeVolume(-5)
				m.setStatus(fmt.Sprintf("Volume: %d%%", vol))
			}
			return m, nil

		case "d", "D":
			m.showDebug = !m.showDebug
			return m, nil
		}
	}

	return m, nil
}

func (m *Model) setStatus(msg string) {
	m.statusMsg = msg
	m.statusTime = time.Now()
}

// View renders the terminal interface
func (m Model) View() string {
	if m.quitting {
		return "\n  Call ended. TermCall disconnected.\n\n"
	}

	themeCyan := lipgloss.Color("#00F0FF")
	themeGreen := lipgloss.Color("#00FF88")
	themeOrange := lipgloss.Color("#FFAA00")
	themeGray := lipgloss.Color("#666666")
	themeDim := lipgloss.Color("#444444")
	themePurple := lipgloss.Color("#BD34FE")

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(themeCyan)

	statusConnectedStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(themeGreen)

	statusWaitingStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(themeOrange)

	statusDisconnectedStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FF3366"))

	labelStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#EEEEEE")).
		Width(12)

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#CCCCCC"))

	hotkeyStyle := lipgloss.NewStyle().
		Foreground(themeCyan).
		Bold(true)

	// Fetch dynamic stats
	connected := false
	peerName := "Connecting..."
	peerEndpoint := "Waiting for handshake..."
	muted := false
	volume := 100
	var micDB, micNorm float64 = -60, 0
	var spkDB, spkNorm float64 = -60, 0
	var rtt time.Duration
	var lossRate float64
	var jitterMs float64
	var bufferMs int

	if m.state != nil {
		connected = m.state.IsConnected()
		peerName = m.state.PeerName()
		peerEndpoint = m.state.PeerEndpoint()
		muted = m.state.IsMuted()
		volume = m.state.Volume()
		micDB, micNorm = m.state.MicLevel()
		spkDB, spkNorm = m.state.SpeakerLevel()
		rtt, lossRate, jitterMs, bufferMs = m.state.Telemetry()
	}

	var b strings.Builder

	// Header Banner
	headerTitle := fmt.Sprintf("TERMCALL v%s · P2P ENCRYPTED AUDIO LINK", Version)
	b.WriteString(titleStyle.Render("  " + headerTitle))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(themeDim).Render(strings.Repeat("─", 74)))
	b.WriteString("\n\n")

	// Call Metadata
	b.WriteString(fmt.Sprintf("  %s %s\n", labelStyle.Render("PEER:"), valueStyle.Render(fmt.Sprintf("%s (%s)", peerName, peerEndpoint))))

	var statusBadge string
	if connected {
		statusBadge = statusConnectedStyle.Render("● Connected (Direct P2P, Zero Relay)")
	} else if strings.Contains(strings.ToLower(peerName), "left") || strings.Contains(strings.ToLower(peerName), "disconnect") {
		statusBadge = statusDisconnectedStyle.Render("○ Peer Disconnected / Call Ended")
	} else {
		statusBadge = statusWaitingStyle.Render("○ Connecting / Punching NAT...")
	}
	b.WriteString(fmt.Sprintf("  %s %s\n", labelStyle.Render("STATUS:"), statusBadge))

	b.WriteString(fmt.Sprintf("  %s %s\n", labelStyle.Render("CRYPTO:"),
		lipgloss.NewStyle().Foreground(themePurple).Render("X25519 · ChaCha20-Poly1305 (256-bit AEAD)")))
	b.WriteString(fmt.Sprintf("  %s %s\n", labelStyle.Render("CODEC:"),
		valueStyle.Render("Opus 48kHz Mono · 24 kbps · 20ms frames")))

	b.WriteString("\n")

	// Live VU Meters
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#AAAAAA")).Render("  AUDIO LEVEL (INPUT):") + "\n")
	micBadge := "MIC "
	if muted {
		micBadge = lipgloss.NewStyle().Foreground(themeOrange).Bold(true).Render("MUT ")
	}
	b.WriteString(fmt.Sprintf("  %s %s %s\n",
		micBadge,
		renderMeter(micNorm, 30, muted),
		renderDB(micDB, muted)))

	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#AAAAAA")).Render("  AUDIO LEVEL (OUTPUT):") + "\n")
	b.WriteString(fmt.Sprintf("  PEER %s %s\n",
		renderMeter(spkNorm, 30, false),
		renderDB(spkDB, false)))

	b.WriteString("\n")

	// Telemetry Row
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#AAAAAA")).Render("  NETWORK TELEMETRY:") + "\n")
	rttColor := themeGreen
	if rtt > 100*time.Millisecond {
		rttColor = themeOrange
	}
	telemetryText := fmt.Sprintf("  RTT Latency: %s  │  Packet Loss: %.1f%%  │  Jitter: %.1fms  │  Buffer: %dms",
		lipgloss.NewStyle().Foreground(rttColor).Bold(true).Render(fmt.Sprintf("%dms", rtt.Milliseconds())),
		lossRate*100,
		jitterMs,
		bufferMs,
	)
	b.WriteString(telemetryText + "\n")

	// Optional Debug panel
	if m.showDebug && m.state != nil {
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(themeGray).Render("  DEBUG: " + m.state.DebugInfo()) + "\n")
	}

	// Status Notification line / Echo Advice Banner
	b.WriteString("\n")
	if time.Since(m.statusTime) < 3*time.Second && m.statusMsg != "" {
		b.WriteString("  " + lipgloss.NewStyle().Foreground(themeCyan).Bold(true).Render("★ "+m.statusMsg) + "\n")
	} else if !connected && (strings.Contains(strings.ToLower(peerName), "left") || strings.Contains(strings.ToLower(peerName), "disconnect")) {
		b.WriteString("  " + lipgloss.NewStyle().Foreground(lipgloss.Color("#FF3366")).Bold(true).Render("⚠️  Peer disconnected or left call. Press [Q] or [ESC] to exit.") + "\n")
	} else {
		b.WriteString("  " + lipgloss.NewStyle().Foreground(themeGray).Render("🎧 Tip: Use headphones for optimal, echo-free conversation.") + "\n")
	}

	// Hotkey Navigation Footer
	b.WriteString(lipgloss.NewStyle().Foreground(themeDim).Render(strings.Repeat("─", 74)))
	b.WriteString("\n  ")

	muteKeyText := "[SPACE] Mute Mic"
	if muted {
		muteKeyText = "[SPACE] Unmute Mic"
	}

	keys := []string{
		fmt.Sprintf("%s %s", hotkeyStyle.Render("[SPACE]"), muteKeyText[8:]),
		fmt.Sprintf("%s %s (%d%%)", hotkeyStyle.Render("[↑/↓]"), "Volume", volume),
	}

	keys = append(keys, fmt.Sprintf("%s %s", hotkeyStyle.Render("[D]"), "Debug"))
	keys = append(keys, fmt.Sprintf("%s %s", hotkeyStyle.Render("[Q/ESC]"), "Hang Up"))

	b.WriteString(strings.Join(keys, "   "))
	b.WriteString("\n")

	// Wrap in handsome terminal border
	boxStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(themeCyan).
		Padding(0, 1)

	return "\n" + boxStyle.Render(b.String()) + "\n"
}

// renderMeter builds dynamic ASCII bars: [■■■■■■■■■□□□□□□□□□□□]
func renderMeter(norm float64, width int, muted bool) string {
	if muted || norm <= 0.0 {
		return "[" + lipgloss.NewStyle().Foreground(lipgloss.Color("#333333")).Render(strings.Repeat("□", width)) + "]"
	}

	filled := int(norm * float64(width))
	if filled > width {
		filled = width
	}
	empty := width - filled

	filledStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF88"))
	if norm > 0.8 {
		filledStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF3366")) // red warning peak
	} else if norm > 0.5 {
		filledStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFAA00")) // yellow
	}

	emptyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#333333"))

	return "[" + filledStyle.Render(strings.Repeat("■", filled)) + emptyStyle.Render(strings.Repeat("□", empty)) + "]"
}

func renderDB(dB float64, muted bool) string {
	if muted {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#FFAA00")).Render("MUTED")
	}
	if dB <= -58.0 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#555555")).Render("-inf dB")
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#CCCCCC")).Render(fmt.Sprintf("%+.0f dB", dB))
}
