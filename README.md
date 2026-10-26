# 🎙️ TermCall

**Direct, Peer-to-Peer Encrypted Voice Calls in your Terminal Shell.**

No browser. No Electron. No accounts. No server-side audio relay.  
A single Go binary that establishes an end-to-end encrypted, low-latency, crystal-clear voice link directly between terminal prompts in under 3 seconds.

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

## ⚡ The Origin Synergy

TermCall synthesizes two engineering crafts:
1. **[MyVault](https://github.com/rakshithgoud453/myvault)**: Zero-telemetry, local-first cryptography, clean Unix CLI ergonomics, and ephemeral memory security.
2. **[Converze Club](https://converze.org)**: Real-time telephony and voice practice for language learners worldwide.

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

### 2. Architectural Boundaries & Converze Separation
TermCall is strictly the **P2P encrypted audio engine**. High-level applications such as [Converze](https://converze.org) (anonymous matchmaking, topic queues, and moderation) sit entirely on top of TermCall as separate modules. 

For the complete architectural separation of concerns, see:
👉 **[Architecture Boundaries: TermCall vs. Converze](docs/ARCHITECTURE_BOUNDARIES.md)**

---

## 🚀 Quickstart & Usage

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

### 4. Diagnostics
```bash
# List local microphone & speaker hardware devices
termcall devices

# Discover external public IP and UDP port mapping via RFC 5389 STUN
termcall stun
```

---

## 🛡️ Security & Privacy Guarantees

- **Ephemeral Key Exchange**: Fresh X25519 ECDH keypairs are generated in RAM for every session.
- **Directional AEAD Keys**: Directional keys are derived using HKDF-SHA256 (`host-to-peer` and `peer-to-host`), completely preventing reflection attacks.
- **Anti-Replay Sliding Window**: Monotonic 64-bit sequence counters and RFC 6479 sliding bitmaps drop replayed or delayed packets.
- **Memory Scrubbing**: Sensitive key material and shared secrets are wiped from memory on call teardown.
- **Zero Plaintext**: ISP or Wi-Fi packet sniffers observe only random high-entropy bytes.

---

## 📦 Building from Source

### Prerequisites
- macOS (CoreAudio) or Linux (ALSA / PulseAudio / PipeWire)
- Go 1.22+
- `opus` and `opusfile` (`brew install opus opusfile` on macOS)

### Build
```bash
# Compile TermCall standalone binary
go build -o termcall ./cmd/termcall
```

---

## 📜 License

MIT License · Built with craft by **Rakshith Billola** ([rakshith.converze.org](https://rakshith.converze.org/)).
