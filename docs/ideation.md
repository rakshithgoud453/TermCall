# Project Spec: TermCall (P2P Encrypted Terminal Voice Link in Go)

**Working Title:** `termcall` / `termvoice`  
**Philosophy:** No browser. No Electron. No accounts. Real-time encrypted voice right in your terminal shell.  
**Author:** Rakshith Billola ([rakshith.converze.org](https://rakshith.converze.org/))  
**Origin Synergy:** Synthesizes the real-time audio/telephony expertise of **Converze Club** with the local-first, zero-telemetry systems craft of **MyVault**.  
**Status:** In Ideation / Priority Architectural Draft  

---

## 1. Vision & The Core Product

### The Friction We Are Eliminating
When two developers are pairing, debugging a production incident, or reviewing a diff in terminal, jumping on a voice call is filled with friction:
- Opening Slack Huddles, Discord, or Google Meet consumes 600MB+ of RAM.
- Creating a meeting link, copying the URL, waiting for browser permissions, and signing into corporate identity providers.
- Cloud platforms record telemetry, route voice through centralized servers, and require account registrations.

### The Product: `termcall`
A single static CLI binary in Go that lets two engineers start an end-to-end encrypted, low-latency, crystal-clear voice call directly between their terminal prompts in 3 seconds:

```bash
# Developer A (Host)
$ termcall host
Room Token: tc_7f91a27e-398b  [Copied to clipboard]
Listening on UDP with STUN hole-punching...
Waiting for peer...

# Developer B (Join)
$ termcall join tc_7f91a27e-398b
Handshake complete. P2P direct UDP connection established!
Connected to peer.
```

---

## 2. Interactive Terminal UI (TUI) Experience

Once connected, `termcall` enters an ultra-slick, minimal terminal interface rendered with Go's `bubbletea` and `lipgloss`:

```
┌────────────────────────────────────────────────────────────────────────┐
│  TERMCALL v0.1.0 · P2P ENCRYPTED AUDIO LINK                            │
├────────────────────────────────────────────────────────────────────────┤
│                                                                        │
│  PEER:      remote-dev (direct UDP 142.250.190.46:51820)               │
│  STATUS:    ● Connected (Direct P2P, Zero Relay)                       │
│  CRYPTO:    X25519 · ChaCha20-Poly1305 (256-bit AEAD)                 │
│  CODEC:     Opus 48kHz Stereo · 24 kbps · 20ms frames                 │
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

## 3. Deep Architectural Schematics

### End-to-End Media & Networking Flow

```
┌───────────────────────── Host (Dev A) ─────────────────────────┐
│                                                                │
│ [Microphone] ──► MiniAudio / PCM (48kHz, 16-bit)               │
│                        │                                       │
│                        ▼                                       │
│               Opus Audio Encoder (20ms frames, ~24kbps)        │
│                        │                                       │
│                        ▼                                       │
│          ChaCha20-Poly1305 AEAD Packet Encryptor               │
│          (Key: X25519 Shared Secret, Nonce: 64-bit Seq#)       │
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
│          ChaCha20-Poly1305 Packet Decryptor & Auth             │
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

## 4. Engineering Mechanisms: Why This Wins

### A. Low-Latency Audio: MiniAudio + Opus
- **Hardware Capture & Playback**: Powered by `malgo` (Go bindings for `miniaudio`), a header-only, zero-dependency C library that interacts directly with:
  - macOS: CoreAudio
  - Linux: ALSA / PulseAudio / PipeWire
  - Windows: WASAPI
- **Opus Compression**:
  - Compresses uncompressed 48,000 Hz, 16-bit PCM audio (768 kbps) down to **24–32 kbps** with zero perceptible loss in human speech quality.
  - Generates **20ms audio frames** (960 samples per frame). Algorithmic encoding delay is only ~5ms.

### B. Direct P2P NAT Hole-Punching (STUN & ICE)
How do two machines connect without port forwarding or a paid middleman server?
1. Both peers query a public STUN server (e.g. `stun.l.google.com:19302` or `stun.cloudflare.com:3478`) to discover their external public IP and UDP port mapping.
2. A lightweight, ephemeral rendezvous signaling server (or a simple open WebSocket broker) swaps the initial IP:Port credentials and public keys between Dev A and Dev B.
3. Both peers immediately send UDP punch packets directly to each other's public endpoints.
4. Once NAT mapping opens, the signaling connection closes. **100% of audio flows directly between the two laptops.**

### C. Military-Grade End-to-End Cryptography
- **Key Exchange**: Ephemeral X25519 Elliptic Curve Diffie-Hellman (ECDH) exchange happens on call setup. The rendezvous server never sees or holds private keys.
- **Packet Encryption**: Every single 20ms UDP datagram is encrypted using **ChaCha20-Poly1305 AEAD**:
  - Authenticated: Tampering with audio packets causes immediate MAC drop.
  - Monotonic Nonces: Nonces are derived from a 64-bit packet counter, completely neutralizing replay attacks.
  - Zero Plaintext: If packets are intercepted by an ISP or Wi-Fi sniffer, only high-entropy random bytes are visible.

### D. Adaptive Jitter Buffer & Packet Loss Concealment (PLC)
UDP does not retransmit dropped packets (retransmission adds unacceptable latency in real-time conversation).
- **Jitter Buffer**: An in-memory ring buffer holds 20–40ms of audio frames to absorb internet jitter.
- **Opus PLC**: If a packet is dropped, the Opus decoder intelligently predicts and synthesizes the missing 20ms waveform from surrounding frames, preventing audio clicks and stutter.

---

## 5. Technical Comparison: Why It Blows Away Existing Tools

| Metric / Dimension | `termcall` | Slack Huddles | Discord | Zoom |
| :--- | :---: | :---: | :---: | :---: |
| **Interface** | **Terminal CLI (TUI)** | Heavy Electron App | Heavy Electron App | Native/Electron App |
| **RAM Footprint** | **~18 MB** | ~650 MB | ~500 MB | ~400 MB |
| **Account Required?** | ❌ **None (Zero)** | ✅ Work Account | ✅ Discord Account | ✅ Zoom Account |
| **Media Routing** | 🚀 **100% Direct P2P** | 🏢 Slack SFU Relay | 🏢 Discord Gateway | 🏢 Zoom Cloud |
| **End-to-End Encryption** | ✅ **X25519 + ChaCha20** | ⚠️ Server-side Decrypted | ⚠️ Gateway Decrypted | ⚠️ Optional / Cloud |
| **Startup to Audio Time** | **< 3 seconds** | 15–30 seconds | 15–20 seconds | 30+ seconds |
| **Binary Portability** | **Single static Go binary** | Requires full OS install | Requires full OS install | Requires installer |

---

## 6. Phased Implementation Roadmap

### Milestone 1: The Local Audio Loopback (2–3 Days)
- Initialize `malgo` audio capture device (Microphone).
- Feed raw PCM buffers into Opus encoder.
- Decode encoded Opus packets back to PCM and play through speakers.
- Validate audio quality, sample rates (48kHz), and latency (<10ms local delay).

### Milestone 2: Localhost UDP Audio Pipeline (2 Days)
- Spin up two CLI processes on localhost (`termcall host --port 5000` & `termcall join localhost:5000`).
- Stream Opus packets over UDP sockets.
- Build the ring buffer and packet sequencing.

### Milestone 3: End-to-End Encryption Layer (2 Days)
- Integrate X25519 key exchange on connect.
- Wrap packet transmission in ChaCha20-Poly1305 AEAD.
- Implement replay prevention window using 64-bit sequence counters.

### Milestone 4: NAT Traversal & Ephemeral Rendezvous (3–4 Days)
- Implement STUN client in Go (`pion/stun`).
- Create an open-source, 50-line ephemeral signaling relay (can run on Cloudflare Workers or a tiny Go server on your Raspberry Pi).
- Exchange SDP/ICE candidate endpoints and execute UDP hole-punching.

### Milestone 5: The Terminal UI & Polish (3 Days)
- Build the `bubbletea` TUI:
  - Real-time ASCII volume VU meters.
  - Hotkey handlers (`<Space>` mute, volume adjustments, hang up).
  - Network telemetry display (RTT ping, packet loss counter, bitrate).

---

## 7. The Demonstration Factor ("The Wow Effect")

Imagine this demo video on Twitter, LinkedIn, and your personal portfolio:

1. **Split-Screen Terminal Demo**:
   - Left side: Rakshith typing `termcall host`.
   - Right side: A remote collaborator typing `termcall join <token>`.
2. **Instant Connection**:
   - The TUI flashes green: `● Connected (Direct P2P, RTT: 28ms)`.
3. **Live Audio Visualizer**:
   - Rakshith speaks into his MacBook mic: the ASCII bars bounce dynamically in real-time.
   - The remote peer answers through the speakers with zero background delay.
4. **Conclusion**:
   - *"Built in Go. 18MB RAM. No accounts. Direct P2P audio. Check it out on GitHub."*

This will immediately position you as a top-tier systems engineer who can tame hardware audio, networking protocols, cryptography, and terminal interfaces.
