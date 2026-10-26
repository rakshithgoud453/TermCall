<div align="center">

# 🎙️ TermCall

**Direct, Peer-to-Peer Encrypted Voice Calls in your Terminal Shell.**  
*No browser. No Electron. No accounts. Zero audio bandwidth bills. Single static Go binary.*

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat-square&logo=go)](https://go.dev/)
[![Audio Codec](https://img.shields.io/badge/Audio-Opus%2048kHz%20VoIP-4A154B?style=flat-square)](https://opus-codec.org/)
[![Crypto](https://img.shields.io/badge/Crypto-X25519%20%2B%20ChaCha20--Poly1305-blue?style=flat-square)](https://en.wikipedia.org/wiki/ChaCha20-Poly1305)
[![Transport](https://img.shields.io/badge/Transport-P2P%20UDP%20%2B%20STUN%20(RFC%205389)-success?style=flat-square)](https://datatracker.ietf.org/doc/html/rfc5389)
[![Replay Protection](https://img.shields.io/badge/Security-RFC%206479%20Anti--Replay-green?style=flat-square)](https://datatracker.ietf.org/doc/html/rfc6479)
[![License](https://img.shields.io/badge/License-MIT-orange?style=flat-square)](LICENSE)
[![Author](https://img.shields.io/badge/Author-Rakshith%20Billola-d7ff3f?style=flat-square&labelColor=171812)](https://rakshith.converze.org/)

---

### 📖 [Read the Comprehensive Engineering Deep-Dive (23 KB)](docs/ENGINEERING_DEEP_DIVE.md)
*An exhaustive breakdown covering hardware PCM capture via MiniAudio, Opus VoIP frame slicing, RFC 5389 STUN NAT traversal, directional AEAD key derivation via HKDF-SHA256, and RFC 6479 sliding-bitmap replay prevention.*

👉 **[Architecture Boundaries: TermCall vs. Converze](docs/ARCHITECTURE_BOUNDARIES.md)** · **[User Manual & Commands](docs/USER_MANUAL.md)**

---

</div>

```
┌────────────────────────────────────────────────────────────────────────┐
│  TERMCALL v0.1.0 · P2P ENCRYPTED AUDIO LINK                            │
├────────────────────────────────────────────────────────────────────────┤
│                                                                        │
│  PEER:      remote-dev (direct UDP 142.250.190.46:51820)               │
│  STATUS:    ● Connected (Direct P2P, Zero Relay)                       │
│  CRYPTO:    X25519 · ChaCha20-Poly1305 (256-bit AEAD)                 │
│  CODEC:     Opus 48kHz Mono · 24 kbps · 20ms frames                    │
│                                                                        │
│  AUDIO LEVEL (INPUT):                                                  │
│  MIC  [■■■■■■■■■■■■■■■□□□□□□□□□□□□□□□] -18 dB                         │
│                                                                        │
│  AUDIO LEVEL (OUTPUT):                                                 │
│  PEER [■■■■■■■■■■■■■■■■■■■■□□□□□□□□□□] -12 dB                         │
│                                                                        │
│  NETWORK TELEMETRY:                                                    │
│  RTT Latency: 22ms  │  Packet Loss: 0.0%  │  Jitter Buffer: 18ms       │
│                                                                        │
├────────────────────────────────────────────────────────────────────────┤
│  [SPACE] Mute Mic   [↑/↓] Volume   [D] Network Debug   [Q/ESC] Hang Up │
└────────────────────────────────────────────────────────────────────────┘
```

---

## 💡 The Philosophy: Why TermCall?

When developers pair-program, troubleshoot production incidents, or review a pull request, jumping on voice should take **under 3 seconds** without pulling focus from the terminal.

Instead, modern collaboration forces high friction:
- ❌ **Massive Bloat**: Opening Slack Huddles, Discord, or Zoom consumes 600 MB+ to 1.5 GB of RAM.
- ❌ **Identity & Accounts**: Mandatory corporate logins, meeting links, browser permissions, and cloud telemetry.
- ❌ **Centralized Relays**: Cloud providers route and decode audio through remote servers, creating privacy risks and massive bandwidth costs.

**TermCall** eliminates the middleman with Unix minimalism:

| Guarantee / Metric | TermCall | Slack Huddles | Discord | Zoom |
| :--- | :---: | :---: | :---: | :---: |
| **Interface** | 💻 **Terminal TUI** | Heavy Electron App | Heavy Electron App | Native/Electron App |
| **RAM Consumption** | ⚡ **~18 MB** | ~650 MB | ~500 MB | ~400 MB |
| **Account Required?** | ❌ **Zero (None)** | ✅ Work Account | ✅ Discord Account | ✅ Zoom Account |
| **Media Routing** | 🚀 **100% Direct P2P** | 🏢 Slack SFU Relay | 🏢 Discord Gateway | 🏢 Zoom Cloud |
| **End-to-End Encryption** | 🔐 **X25519 + ChaCha20** | ⚠️ Server-Decrypted | ⚠️ Gateway-Decrypted | ⚠️ Optional / Cloud |
| **Startup to Audio Time** | ⚡ **< 3 seconds** | 15–30 seconds | 15–20 seconds | 30+ seconds |
| **Binary Portability** | 📦 **Single Go Binary** | Full OS Package | Full OS Package | Full OS Installer |
| **Audio Bandwidth Bill** | 🟢 **$0.00 / month** | Paid Enterprise | Paid Nitro / Server | Paid Enterprise |

---

## ⚡ The Origin Synergy

TermCall is born from the intersection of two systems engineering crafts:
1. **[MyVault](https://github.com/rakshithgoud453/myvault)**: Zero-telemetry, local-first cryptography, clean Unix CLI ergonomics, and ephemeral memory security.
2. **[Converze Club](https://converze.org)**: Telephony routing, audio intelligence, and voice practice for language learners worldwide.

By coupling hardware-level audio capture with direct UDP NAT hole-punching and military-grade cryptography, TermCall enables direct communication without paying a dime in audio bandwidth bills.

---

## 🏗️ Architecture Schematics

### 1. Media & Cryptographic Pipeline
```
┌───────────────────────── Host (Dev A) ─────────────────────────┐
│                                                                │
│ [Microphone] ──► MiniAudio / PCM (48kHz, 16-bit Mono)          │
│                        │                                       │
│                        ▼                                       │
│               Opus Audio Encoder (20ms frames, ~24 kbps)       │
│                        │                                       │
│                        ▼                                       │
│          ChaCha20-Poly1305 AEAD Packet Encryptor               │
│          (Key: Ephemeral X25519 ECDH, Nonce: 64-bit Seq#)      │
│                        │                                       │
│                        ▼                                       │
│                  UDP Datagram Socket                           │
└────────────────────────┬───────────────────────────────────────┘
                         │
                         ▼ (Encrypted UDP Packets over Public Internet)
              [ NAT Traversal / STUN Hole-Punch ]
                         │
┌────────────────────────┴───────────────────────────────────────┐
│                  UDP Datagram Socket                           │
│                        │                                       │
│                        ▼                                       │
│          ChaCha20-Poly1305 Packet Decryptor & Anti-Replay      │
│                        │                                       │
│                        ▼                                       │
│               Adaptive Jitter Buffer (20–40ms)                 │
│                        │                                       │
│                        ▼                                       │
│               Opus Audio Decoder + Packet Loss Concealment     │
│                        │                                       │
│                        ▼                                       │
│ [Speakers]   ◄── MiniAudio / PCM Playback                      │
│                                                                │
└──────────────────────── Peer (Dev B) ──────────────────────────┘
```

---

### 2. The Wire Protocol Binary Framing

Every 20ms, TermCall transmits a compact, tamper-proof UDP datagram. Overhead is kept strictly to **16 bytes**:

```
 0                   1                   2                   3
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|   'T' (0x54)  |   'C' (0x43)  |  Version(0x01)| PacketType(1B)|
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                 64-bit Monotonic Packet Sequence #            |
|                 (Acts as Nonce for ChaCha20-Poly1305)         |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|       Timestamp (uint16)      |     Payload Length (uint16)   |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                                                               |
|        Encrypted Payload + Poly1305 Authentication Tag        |
|        (Opus Audio Frame / Handshake Data / Keepalive)        |
|                                                               |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

* **Packet Types**: `0x01` (Audio), `0x02` (Handshake), `0x03` (HandshakeAck), `0x04` (Ping), `0x05` (Pong), `0x06` (Mute), `0x07` (Hangup).

---

### 3. Serverless Dual-Candidate Room Token

TermCall requires **zero external signaling servers** to pair two callers. The room token encodes both candidates plus the host's public key in a compact 44-byte Base64 URL:

```
~ Room Token Structure: tc_<base64> ~
[ 4B Public IP ] + [ 2B Public Port ] + [ 4B LAN IP ] + [ 2B LAN Port ] + [ 32B X25519 PubKey ]
```

- **LAN Auto-Detection**: If callers are on the same Wi-Fi network, TermCall routes packets directly through LAN UDP without traversing the public internet router.
- **WAN STUN Hole-Punching**: If callers are remote, TermCall bursts handshake datagrams to the STUN reflexive address to punch NAT pinholes simultaneously.

---

## 🛡️ Security & Cryptographic Invariants

1. **Ephemeral Key Exchange (X25519 ECDH)**:
   - Fresh 256-bit elliptic curve keypairs are generated in physical RAM for each call session.
   - Private keys never touch storage disks and are securely zeroed out on teardown.
2. **Directional Key Derivation (HKDF-SHA256)**:
   - Derives two distinct symmetric keys: `termcall-host-to-peer` and `termcall-peer-to-host`.
   - Prevents **packet reflection attacks** (an attacker bouncing a caller's own encrypted packets back to them).
3. **Anti-Replay Protection (RFC 6479 Sliding Bitmap)**:
   - Sequence numbers monotonically increment.
   - An in-memory 128-packet sliding window drops replayed, duplicated, or excessively delayed datagrams.
4. **Zero-Knowledge Privacy**:
   - Audio is authenticated and encrypted directly between end-devices using ChaCha20-Poly1305.
   - Third-party observers, ISPs, or Wi-Fi sniffers see only high-entropy random UDP bytes.

---

## 🚀 Quickstart & Commands

### 1. Test Hardware Audio Locally (Loopback Mode)
Test your microphone, speakers, Opus compression, and ChaCha20 encryption instantly on your local machine:
```bash
termcall loopback
```
Speak into your microphone—you will hear your voice with ~20ms delay and see the real-time ASCII VU meter bounce!

### 2. Host a 1-on-1 Call
```bash
termcall host
```
Outputs a compact, serverless room token (e.g. `tc_jvq-LspsAAc...`) containing the host's public STUN reflexive address and ephemeral X25519 public key.

### 3. Join a Call
```bash
termcall join tc_jvq-LspsAAc...
```
Automatically executes direct UDP NAT hole-punching, establishes symmetric AEAD encryption, and launches the live TUI.

### 4. Hardware & Network Diagnostics
```bash
# List local microphone & speaker hardware devices
termcall devices

# Discover external public IP and UDP port mapping via RFC 5389 STUN
termcall stun

# Display version
termcall version
```

---

## ⌨️ Interactive TUI Controls

While in a call, control your audio and monitor telemetry via intuitive keyboard shortcuts:

| Key | Action |
| :---: | :--- |
| `<Space>` | **Toggle Microphone Mute / Unmute** (Transmits `TypeMute` packet to peer) |
| `↑` / `↓` | **Adjust Speaker Volume** (0% to 200% with soft-clipping) |
| `d` / `D` | **Toggle Network Telemetry HUD** (RTT, Jitter buffer depth, Packet stats) |
| `q` / `<Esc>` | **Hang Up** (Sends graceful `TypeHangup` teardown packet and exits cleanly) |

---

## 📦 Building from Source

### Prerequisites
- macOS (CoreAudio) or Linux (ALSA / PulseAudio / PipeWire)
- Go 1.22+
- `opus` and `opusfile` development libraries:
  - **macOS**: `brew install opus opusfile`
  - **Debian / Ubuntu**: `sudo apt-get install libopus-dev libopusfile-dev`
  - **Fedora**: `sudo dnf install opus-devel opusfile-devel`

### Build
```bash
# Clone repository
git clone https://github.com/rakshithgoud453/termcall.git
cd termcall

# Run test suite across all subsystems
go test ./...

# Build standalone static binary
go build -o termcall ./cmd/termcall
```

---

## 🌐 The Converze Connection

TermCall is engineered as an independent, high-performance **P2P audio engine**. High-level applications—such as **[Converze Club](https://converze.org)** (anonymous matchmaking, topic queues, and community language practice)—build directly on top of TermCall's transport layer.

For the architectural separation of concerns, see:  
👉 **[Architecture Boundaries: TermCall vs. Converze](docs/ARCHITECTURE_BOUNDARIES.md)**  
👉 **[Converze P2P Matchmaker Spec](docs/03_converze_p2p_matchmaker.md)**

---

## 📄 License & Attribution

- Built with craft by **Rakshith Billola** — Systems & Software Engineer ([rakshith.converze.org](https://rakshith.converze.org/))
- Licensed under the **MIT License**. See [LICENSE](LICENSE) for details.
- Security Policy & Disclosures: [SECURITY.md](SECURITY.md)
