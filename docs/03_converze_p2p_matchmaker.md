# Project Spec: Converze P2P Matchmaker (Anonymous Voice Layer Built on TermCall)

**Working Title:** `converze-match` / `converze-core`  
**Parent Engine:** Powered by the **`termcall`** P2P UDP & Opus audio engine  
**Philosophy:** Zero-knowledge anonymity. Zero audio servers. Frictionless voice practice for English learners worldwide.  
**Author:** Rakshith Billola ([rakshith.converze.org](https://rakshith.converze.org/)) · Co-creator of Converze Club ([converze.org](https://converze.org/))  
**Status:** In Ideation / Application Layer Specification  

---

## 1. Vision & Architecture Overview

### The Goal
Converze Club (`converze.org`) empowers English speaking enthusiasts and learners to overcome language hesitation through private, anonymous, real-time audio conversations.

By building on top of the **`termcall` P2P audio engine**, Converze achieves what no traditional voice platform can:
- **Zero Audio Bandwidth Bills ($0/month):** Media streams directly between peers over UDP. No audio ever routes through a central server.
- **Absolute Cryptographic Anonymity:** Calls are end-to-end encrypted with ephemeral X25519 keys generated in RAM per call. Even the Converze server cannot listen, record, or inspect speech.
- **Cross-Platform Compatibility:** Web users practicing on `converze.org` and terminal users practicing via `termcall match` can be paired seamlessly over standard Opus/STUN channels.

---

## 2. System Architecture: The Matchmaker Separation

```
[ User A (Caller) ]                             [ User B (Caller) ]
  (Browser or CLI)                                (Browser or CLI)
         │                                                │
         │ 1. Connects to Matchmaker                      │ 1. Connects to Matchmaker
         │    {"topic": "english", "level": "intermediate"}│    {"topic": "english", "level": "intermediate"}
         ▼                                                ▼
┌────────────────────────────────────────────────────────────────────────┐
│                   CONVERZE BLIND MATCHMAKER                            │
│           (Hosted on Raspberry Pi / Cloudflare Worker)                  │
│                                                                        │
│   • In-Memory Matchmaking Queue (by Topic & Language Level)            │
│   • Pairs User A & User B blindly in FIFO order                        │
│   • Relays STUN Public Endpoints + Ephemeral X25519 Public Keys        │
│   • Immediately drops the session coordinator                         │
└───────────────────────────────────┬────────────────────────────────────┘
                                    │
                                    │ 2. Introductions Sent (~200 bytes JSON)
                                    ▼
       ┌─────────────────────────────────────────────────────────┐
       │   TERMCALL P2P ENGINE (Direct UDP NAT Hole-Punch)       │
       │                                                         │
       │   User A ◄════════════════════════════════════► User B  │
       │   (ChaCha20-Poly1305 Encrypted 48kHz Opus Audio)        │
       └─────────────────────────────────────────────────────────┘
```

---

## 3. The Matchmaker Protocol (Signaling Flow)

The matchmaker does **not** handle WebRTC media or audio streams. It is an ultra-lightweight signaling broker (WebSockets or HTTP Long-Polling).

### Step 1: The Match Request (Client → Server)
When a user clicks "Start Practice" on the web or runs `termcall match`, the client queries public STUN (`stun.l.google.com:19302`) and sends its discovery payload:

```json
{
  "event": "queue_join",
  "topic": "daily_conversations",
  "level": "intermediate",
  "public_endpoint": "142.250.190.46:54120",
  "public_key": "x25519_pub_78af9c20..."
}
```

### Step 2: In-Memory Pairing Logic (Server)
The matchmaker maintains FIFO buckets in Go:

```go
type QueueKey struct {
    Topic string
    Level string
}

var queue = make(map[QueueKey][]*PeerSession)
```

1. Server checks if another peer is waiting in `queue[QueueKey]`.
2. **If queue is empty**: Inserts client into queue, sets a 45-second matching timeout.
3. **If peer exists**: Pops the waiting peer, generates a unique ephemeral `session_id`, and immediately sends the reciprocal handshake to both clients.

### Step 3: The Introduction (Server → Both Clients)
Each peer receives the other peer's public address and key:

```json
{
  "event": "peer_matched",
  "session_id": "sess_89a1f2b",
  "peer_endpoint": "104.28.19.88:48231",
  "peer_public_key": "x25519_pub_31b0e4a..."
}
```

### Step 4: P2P Takeover (TermCall Engine)
1. Both clients send UDP punch packets directly to each other's endpoints.
2. Derives shared symmetric secret: `SharedSecret = X25519(MyPrivateKey, PeerPublicKey)`.
3. Audio begins streaming with zero server relay.

---

## 4. User Experience & Flows

### Mode A: Terminal Interface (`termcall match`)
```bash
$ termcall match --topic english

● Connected to Converze matchmaker...
● Looking for someone to practice English with...
✔ Match found! Peer is from: GMT+5:30 (Anonymous)
● Punching UDP tunnel... P2P connected.

┌───────────────────────────────────────────────────────────┐
│ CONVERZE CLUB · ANONYMOUS ENGLISH CALL                    │
│ Status: Connected (Direct P2P, RTT: 31ms)                 │
│ Audio:  Opus 48kHz · E2E Encrypted                        │
│                                                           │
│ [SPACE] Mute   [N] Next Peer (Skip)   [Q] End Call        │
└───────────────────────────────────────────────────────────┘
```

### Mode B: Web Interface (`converze.org`)
- Simple browser UI built with Vanilla JS or React.
- Native browser WebRTC audio captures the mic, encodes Opus, and transmits directly over UDP to the peer (browser-to-browser or browser-to-terminal).

---

## 5. Privacy & Zero-Knowledge Guarantees

1. **No Account Required:** Users join anonymously. No email, phone number, or Google auth is required to practice talking.
2. **Key Forward Secrecy:** A new X25519 keypair is generated in memory for every individual call. When the user disconnects, the private key is wiped via `memguard`. Past calls can never be retroactively decrypted.
3. **No Audio Logs:** Audio packets never touch the matchmaker or any server disk. Audio exists only as transient sound waves leaving the speaker.

---

## 6. Abuse Prevention & Moderation in an Anonymous P2P Network

A common question for anonymous voice apps: *How do you prevent harassment or trolls without recording audio?*

Because Converze does not record voice, it relies on **reputation counters and behavioral signaling**:

1. **Instant Skip / Next Peer (`[N]` key)**: If a caller feels uncomfortable, pressing `Next` instantly severs the P2P connection and drops the abusive peer.
2. **Reputation Score**:
   - Short calls (< 15 seconds) where one peer immediately hangs up or reports degrade the abusive user's ephemeral trust score.
   - Users with high skip rates are throttled in the matchmaking queue (cooldown timer: 5 minutes, 15 minutes, 1 hour).
3. **IP Hash Rate Limiting**: The matchmaker hashes client IPs using HMAC-SHA256 with a daily rotating salt. If an IP generates repeated rapid disconnects or reports, it is blocked from joining the matchmaking queue.

---

## 7. Infrastructure & Cost Breakdown

| Component | Technology | Hosting | Cost |
| :--- | :--- | :--- | :---: |
| **Matchmaker Service** | Go WebSocket / HTTP server (`converze-match`) | Raspberry Pi (`billolapi`) or Cloudflare Workers | **$0.00** |
| **STUN Discovery** | RFC 5389 NAT discovery | Google / Cloudflare public STUN servers | **$0.00** |
| **Media Transmission** | P2P UDP datagrams (`termcall` / WebRTC) | Direct between users' home connections | **$0.00** |
| **Frontend Web** | Static HTML / JS on `converze.org` | Raspberry Pi Nginx / Cloudflare Pages | **$0.00** |
| **Total Monthly Cost** | | | **$0.00** |

---

## 8. Development Roadmap

- **Phase 1 (The Core):** Complete the standalone `termcall` binary (`host` / `join` 1-on-1 calls).
- **Phase 2 (The Matchmaker):** Build a 150-line Go matchmaking coordinator (`matchmaker.go`) running on your Raspberry Pi.
- **Phase 3 (CLI Integration):** Add the `termcall match` command to interface with the matchmaker.
- **Phase 4 (Web Gateway):** Integrate the same matchmaking protocol into the `converze.org` web frontend so web users and CLI users can talk to each other anonymously.
