# Architecture Boundaries: TermCall vs. Converze

> **Core Axiom:** `TermCall` is an infrastructure-level, general-purpose P2P encrypted audio engine. `Converze` is an application-level anonymous voice platform that consumes TermCall. They are completely separate software systems with strictly enforced architectural boundaries.

---

## 1. Executive Summary & Philosophy

When building complex network software, conflating the **transport/media engine** with the **application business logic** leads to bloat, security hazards, and architectural rot.

```
┌────────────────────────────────────────────────────────────────────────┐
│                        CONVERZE CLUB APPLICATION                       │
│    (Anonymous voice practice, topics, levels, matchmaking, trust)      │
│                     Repo / Product: converze.org                       │
└───────────────────────────────────┬────────────────────────────────────┘
                                    │ Consumes via Handoff Contract
                                    ▼
┌────────────────────────────────────────────────────────────────────────┐
│                        TERMCALL AUDIO ENGINE                           │
│     (Microphone, Speakers, Opus, ChaCha20-Poly1305, UDP NAT P2P)       │
│                     Repo / Product: termcall                           │
└────────────────────────────────────────────────────────────────────────┘
```

- **TermCall** adheres strictly to the Unix philosophy: *Do one thing, and do it with unmatched performance and security.* It enables two terminal endpoints to talk to each other over encrypted UDP with zero middleman servers.
- **Converze** is a user-facing product designed to eliminate language hesitation for English speakers worldwide. It handles matchmaking queues, user preferences, trust scores, browser WebRTC clients, and introduces callers before stepping out of the way.

---

## 2. Boundary Delineation Matrix

| Architectural Domain | `TermCall` (The Engine) | `Converze` (The Application) |
| :--- | :--- | :--- |
| **Primary Responsibility** | Real-time audio capture, encoding, encryption, NAT traversal, and low-latency playback. | Anonymous stranger matching, fluency filtering, session moderation, and Web/CLI coordination. |
| **Software Type** | Standalone CLI binary & reusable Go audio library. | Distributed web platform (`converze.org`) + signaling services (`converze-match`). |
| **Networking Scope** | Raw UDP datagram sockets, RFC 5389 STUN NAT hole-punching, packet sequencing, RTT latency ping/pong. | WebSockets / HTTP signaling, in-memory FIFO queues, session orchestration. |
| **Audio & Hardware** | macOS CoreAudio (miniaudio), Linux ALSA/Pulse/PipeWire, Windows WASAPI, 48kHz PCM buffers. | Zero audio handling. Does not touch or decode PCM audio. |
| **Audio Codec** | Opus VoIP (20ms frames, 24 kbps, In-Band FEC, Packet Loss Concealment via `DecodePLC`). | Agnostic to raw Opus packets; assumes standard WebRTC Opus compatibility. |
| **Cryptography** | Ephemeral X25519 ECDH in physical RAM, ChaCha20-Poly1305 AEAD, RFC 6479 anti-replay sliding window. | Facilitates public key discovery during pairing. Never holds private keys. |
| **User Interface** | Bubble Tea + Lip Gloss TUI with real-time ASCII VU meters, volume, and mute controls. | Web frontend (converze.org) or Converze CLI companion interface. |
| **State & Lifecycle** | Transient call session state (Connected, Disconnected, Muted). Terminated on process exit. | Matchmaking queue state, active session IDs, trust counters, rate limits, reputation scores. |
| **Domain Concepts** | Packets, frames, jitter buffer, dB levels, sequence counters, AES/ChaCha keys. | Topics (e.g. `daily_conversations`, `tech`), levels (`beginner`, `advanced`), skips (`[N]`), trust. |

---

## 3. What TermCall Explicitly Does NOT Know About

To preserve architectural purity, the `TermCall` codebase contains **zero** references to:
1. **Matchmaking**: TermCall has no concept of a "queue", "matchmaker", or "finding strangers". Connections are established strictly via explicit host/join handshakes or room tokens (`tc_...`).
2. **Domain Taxonomies**: TermCall knows nothing about "English learning", "topics", "fluency levels", or "conversation prompts".
3. **User Accounts / Profiles**: TermCall has no users, IDs, emails, auth tokens, or telemetry databases.
4. **Signaling Servers**: TermCall does not depend on a central server to function. Two peers on the same LAN or with known public endpoints can connect with 0 external infrastructure.
5. **Moderation & Trust Scores**: TermCall does not evaluate user behavior, track call durations for bans, or hash client IPs.

---

## 4. The Integration Contract (How Converze Consumes TermCall)

Converze uses TermCall as an **underlying voice transport engine**. The integration follows a clean, three-phase handoff contract:

```
┌────────────────────────────────────────────────────────────────────────┐
│ Phase 1: Signaling & Matchmaking (Owned 100% by Converze)              │
│                                                                        │
│  Caller A                                                Caller B      │
│  (CLI / Web)                                           (CLI / Web)     │
│       │                                                     │          │
│       │ 1. Connects: {"topic":"tech", "level":"fluent"}      │          │
│       ▼                                                     ▼          │
│  ┌───────────────────────────────────────────────────────────────┐     │
│  │                Converze Blind Matchmaker                      │     │
│  │  • Enqueues callers in FIFO buckets                           │     │
│  │  • Pairs Caller A and Caller B                                │     │
│  │  • Relays STUN Reflexive Endpoints + Ephemeral Public Keys    │     │
│  └───────────────────────────────┬───────────────────────────────┘     │
│                                  │                                     │
│                                  │ 2. Introductions Dispatched         │
└──────────────────────────────────┼─────────────────────────────────────┘
                                   │
                                   ▼
┌────────────────────────────────────────────────────────────────────────┐
│ Phase 2: Session Handoff to TermCall Engine                            │
│                                                                        │
│  Converze generates a TermCall Room Token or passes the connection     │
│  parameters directly to the TermCall engine:                           │
│                                                                        │
│     Endpoint: 142.250.190.46:51820                                     │
│     PubKey:   x25519_pub_31b0e4a...                                    │
│                                                                        │
│  Converze immediately closes its signaling connection and steps away.  │
└──────────────────────────────────┬─────────────────────────────────────┘
                                   │
                                   ▼
┌────────────────────────────────────────────────────────────────────────┐
│ Phase 3: Direct P2P Media Streaming (Owned 100% by TermCall)           │
│                                                                        │
│  Caller A ◄═════════════════════════════════════════════► Caller B     │
│           • Direct UDP NAT Hole-Punching                               │
│           • X25519 Shared Secret Computation (in RAM)                  │
│           • 48kHz Opus Audio Capture & Playback                        │
│           • ChaCha20-Poly1305 AEAD Packet Encryption                   │
│           • Adaptive Jitter Buffering & Packet Loss Concealment        │
└────────────────────────────────────────────────────────────────────────┘
```

### The Handoff Interface Contract

Converze can drive TermCall through two standardized integration surfaces:

#### Option A: Token-Based Process Invocation (CLI Level)
When the Converze client pairs two callers, it encodes the remote peer's public STUN address and ephemeral X25519 key into a standard TermCall token:
```bash
# Converze spawns or delegates to TermCall
termcall join tc_jvq-LspsAAcOFRwjKjE4P0ZNVFtiaXB3foWMk5qhqK...
```

#### Option B: Go Package Consumption (Library Level)
Converze Go clients import TermCall packages directly:
```go
import (
    "github.com/rakshithgoud453/termcall/internal/session"
)

// Converze passes the matched endpoint and key into TermCall
sess, err := session.NewJoinSession(matchedToken)
if err != nil {
    return err
}
sess.Start()
```

---

## 5. WebRTC Interoperability (Web to Terminal)

Because TermCall uses industry-standard codecs and network protocols:
1. **Audio Codec**: Standard Opus 48,000 Hz, 20ms frame duration (RFC 6716).
2. **NAT Discovery**: Standard RFC 5389 STUN Binding Requests.
3. **Wire Format**: UDP datagrams containing standard Opus payloads.

A web browser client accessing **`converze.org`** using standard WebRTC APIs can interoperate seamlessly with a terminal developer running **`termcall`** through standard WebRTC data/media negotiation without requiring transcoding bridges.

---

## 6. Independent Project Roadmaps

### TermCall Roadmap (The Engine)
- [x] Local hardware audio capture & playback via `malgo` (miniaudio).
- [x] Opus 48kHz VoIP encoding, decoding, and Packet Loss Concealment (PLC).
- [x] Ephemeral X25519 ECDH key exchange with ChaCha20-Poly1305 AEAD.
- [x] RFC 6479 anti-replay sliding window.
- [x] RFC 5389 STUN NAT discovery and direct UDP hole-punching.
- [x] Adaptive jitter ring buffer and telemetry tracking (RTT, packet loss, jitter ms).
- [x] Interactive Bubble Tea TUI with real-time ASCII VU meters.
- [ ] Multi-party mesh conferencing (3–4 peers).
- [ ] Bluetooth headset and virtual audio device auto-switching.

### Converze Roadmap (The Application Layer)
- [ ] Converze blind matchmaker coordinator service (`converze-match`).
- [ ] Topic and fluency level matchmaking FIFO queues.
- [ ] Web frontend on `converze.org` for browser-based voice practice.
- [ ] Behavioral moderation, skip rate limits, and HMAC-SHA256 IP trust scoring.
- [ ] Converze CLI companion (`converze practice --topic daily_conversations`).
