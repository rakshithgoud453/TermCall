package cli

import (
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/rakshithgoud453/termcall/internal/audio"
	"github.com/rakshithgoud453/termcall/internal/network"
	"github.com/rakshithgoud453/termcall/internal/session"
	"github.com/rakshithgoud453/termcall/internal/tui"
)

var Version = "0.1.0"

// RootCmd represents the base command
var RootCmd = &cobra.Command{
	Use:   "termcall",
	Short: "TermCall is a peer-to-peer end-to-end encrypted voice link in your terminal.",
	Long: `TermCall: Direct P2P Encrypted Audio Calls in your Terminal Shell.

No browser. No Electron. No accounts. 100% direct UDP audio with 
X25519 ECDH key exchange, ChaCha20-Poly1305 AEAD, and Opus 48kHz compression.`,
}

func init() {
	RootCmd.AddCommand(hostCmd)
	RootCmd.AddCommand(joinCmd)
	RootCmd.AddCommand(loopbackCmd)
	RootCmd.AddCommand(stunCmd)
	RootCmd.AddCommand(devicesCmd)
	RootCmd.AddCommand(versionCmd)
}

// hostCmd
var hostPort int
var hostCmd = &cobra.Command{
	Use:   "host",
	Short: "Host a new 1-on-1 voice call and generate a room token",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("● Initializing TermCall audio engine and resolving STUN...")
		sess, token, err := session.NewHostSession(hostPort)
		if err != nil {
			return fmt.Errorf("failed to create host session: %w", err)
		}
		defer sess.HangUp()

		fmt.Println("\n" + "─────────────────────────────────────────────────────────────")
		fmt.Printf("  ROOM TOKEN:  %s\n", token)
		fmt.Println("─────────────────────────────────────────────────────────────")
		fmt.Println("  Share this token with your collaborator: termcall join <token>")
		fmt.Println("  Launching interactive terminal interface...")
		fmt.Println("")

		if err := sess.Start(); err != nil {
			return fmt.Errorf("failed to start audio: %w", err)
		}

		p := tea.NewProgram(tui.NewModel(sess), tea.WithAltScreen())
		_, err = p.Run()
		return err
	},
}

func init() {
	hostCmd.Flags().IntVarP(&hostPort, "port", "p", 0, "Local UDP port to bind (default 0 for random ephemeral port)")
}

// joinCmd
var joinCmd = &cobra.Command{
	Use:   "join <room-token>",
	Short: "Join a voice call using a host room token",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		token := args[0]
		fmt.Printf("● Joining room %s...\n", token)

		sess, err := session.NewJoinSession(token)
		if err != nil {
			return err
		}
		defer sess.HangUp()

		if err := sess.Start(); err != nil {
			return fmt.Errorf("failed to start audio: %w", err)
		}

		p := tea.NewProgram(tui.NewModel(sess), tea.WithAltScreen())
		_, err = p.Run()
		return err
	},
}

// loopbackCmd
var loopbackCmd = &cobra.Command{
	Use:   "loopback",
	Short: "Test microphone, Opus codec, ChaCha20 encryption, and speakers locally",
	Long: `Runs an instant local loopback test:
Microphone -> Opus 48kHz Encode -> ChaCha20 AEAD Encrypt -> ChaCha20 Decrypt -> Opus Decode -> Speaker Playback.
Watch your live VU meters bounce in real-time to verify audio hardware.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		sess, err := session.NewLoopbackSession()
		if err != nil {
			return err
		}
		defer sess.HangUp()

		if err := sess.Start(); err != nil {
			return fmt.Errorf("failed to start audio hardware: %w", err)
		}

		p := tea.NewProgram(tui.NewModel(sess), tea.WithAltScreen())
		_, err = p.Run()
		return err
	},
}

// stunCmd
var stunCmd = &cobra.Command{
	Use:   "stun",
	Short: "Discover external public NAT address and port via STUN",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("● Querying public STUN servers (RFC 5389)...")
		addr, err := network.DiscoverPublicEndpoint(nil, 3*time.Second)
		if err != nil {
			return err
		}
		fmt.Printf("✔ Public STUN Endpoint: %s (IP: %s, Port: %d)\n", addr.String(), addr.IP, addr.Port)
		return nil
	},
}

// devicesCmd
var devicesCmd = &cobra.Command{
	Use:   "devices",
	Short: "List detected microphone and speaker audio hardware devices",
	RunE: func(cmd *cobra.Command, args []string) error {
		devices, err := audio.ListDevices()
		if err != nil {
			return err
		}
		fmt.Printf("Detected %d audio device(s):\n\n", len(devices))
		for i, d := range devices {
			fmt.Printf(" [%d] %-10s %s (id: %s)\n", i, "["+d.Type+"]", d.Name, d.ID)
		}
		return nil
	},
}

// versionCmd
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print TermCall version and build information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("termcall version %s\n", Version)
	},
}

// Execute runs the root CLI command
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
