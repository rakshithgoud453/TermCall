# 🎙️ TermCall: User Manual & Collaboration Guide

Welcome to **TermCall** — real-time, peer-to-peer, end-to-end encrypted terminal voice calls with zero third-party audio servers.

---

## ⚡ 1. Pre-Flight Check (Local Verification)

Before calling someone, you can verify your microphone, speakers, and encryption locally in under 3 seconds.

### Step 1: Check Detected Audio Devices
```bash
./termcall devices
```
Expected output:
```text
Detected 2 audio device(s):
 [0] [capture]  MacBook Air Microphone
 [1] [playback] MacBook Air Speakers
```

### Step 2: Run Instant Local Loopback Test
```bash
./termcall loopback
```
- Speak into your microphone.
- You will immediately hear your voice played back through your speakers or headphones (~20ms latency).
- You will see the green/yellow **ASCII VU meter** bouncing dynamically in real time:
  ```text
  AUDIO LEVEL (INPUT):
  MIC  [■■■■■■■■■■■■■■■□□□□□□□□□□□□□□□] -18 dB
  ```
- Press `[SPACE]` to test muting/unmuting.
- Press `[Q]` or `[ESC]` to exit.

---

## 🚀 2. How to Call Between Two Machines (Step-by-Step)

TermCall connects two developers directly over UDP without accounts, logins, or cloud meetings.

### On Machine A (Developer A — The Host):

1. Open your terminal and run:
   ```bash
   ./termcall host
   ```
2. TermCall queries public STUN, generates an ephemeral X25519 encryption keypair in RAM, and outputs:
   ```text
   ● Initializing TermCall audio engine and resolving STUN...

   ─────────────────────────────────────────────────────────────
     ROOM TOKEN:  tc_L_eskrBKwKgc29oJSnUMT3Be29wlKl1fNYWiDhGHu9JBWqCp6uM5SQ15ZwI
   ─────────────────────────────────────────────────────────────
     Share this token with your collaborator: termcall join <token>
     Launching interactive terminal interface...
   ```
3. **What to share**: Simply copy the **Room Token** (the string starting with `tc_...`) and send it to your collaborator via Slack, Discord, Signal, WhatsApp, or email.
4. The terminal will enter the live TUI and wait for Developer B:
   ```text
   STATUS: ○ Connecting / Punching NAT...
   ```

---

### On Machine B (Developer B — The Joiner):

1. Paste the token you received and run:
   ```bash
   ./termcall join tc_L_eskrBKwKgc29oJSnUMT3Be29wlKl1fNYWiDhGHu9JBWqCp6uM5SQ15ZwI
   ```
2. What happens automatically:
   - TermCall extracts the Host's candidates (Public STUN address + Local LAN address) and the Host's X25519 public key.
   - Developer B's machine generates its own ephemeral keypair in RAM.
   - It sends simultaneous UDP punch packets to Developer A.
   - Within 1–2 seconds, the handshake completes.
3. Both terminal screens flash green:
   ```text
   STATUS: ● Connected (Direct P2P, Zero Relay)
   ```
4. Start talking! Audio streams with ultra-low latency directly between your two machines.

---

## 🎮 3. In-Call Controls & Hotkeys

While connected in a call, use these single-key hotkeys:

| Key | Action | Description |
| :---: | :--- | :--- |
| **`[SPACE]`** | **Mute / Unmute** | Instantly zeroes outgoing audio packets and displays `MUTED` in orange. |
| **`[↑]`** or **`[+]`** | **Volume Up** | Increases speaker volume in +5% increments (up to 200%). |
| **`[↓]`** or **`[-]`** | **Volume Down** | Decreases speaker volume in -5% increments (down to 0%). |
| **`[D]`** | **Toggle Debug** | Displays real-time transmitted/received packet counts, local socket, and remote endpoint. |
| **`[Q]`** or **`[ESC]`** | **Hang Up** | Sends call teardown notice, terminates audio devices, and scrubs keys from RAM. |

---

## 🌐 4. Network Topologies: Where Does It Work?

TermCall implements **dual-candidate ICE pairing** to guarantee connections in every environment:

### Topology 1: Two Terminals on the Same Laptop
- If you open two terminal windows on the same machine to test:
  - Terminal 1: `./termcall host`
  - Terminal 2: `./termcall join <token>`
- The connection is established over local loopback in **< 1ms**.

### Topology 2: Two Machines on the Same Local Wi-Fi / Office Network
- Both laptops are connected to the same home or office router.
- TermCall detects the LAN candidate (`192.168.x.x`) and connects directly across the local subnet. No packets need to bounce through the internet.

### Topology 3: Two Machines Across the Public Internet
- Developer A is at home in India; Developer B is in Europe on mobile hotspot.
- TermCall's STUN resolver discovers both peers' reflexive public IP and port mappings (`RFC 5389`).
- UDP hole-punching traverses home NAT routers and establishes a direct P2P link.
- **Audio never touches a relay server.**

---

## 🔒 5. What About Privacy & Security?

- **Zero Audio Relays**: Neither TermCall nor any cloud server routes your sound.
- **End-to-End Encrypted**: Every 20ms audio frame is encrypted with **ChaCha20-Poly1305 AEAD**.
- **Ephemeral RAM Keys**: X25519 keys exist only in physical memory during the call. When you press `[Q]`, keys are overwritten with zeroes. Past calls cannot be retroactively decrypted even if your laptop is inspected later.
- **Anti-Replay Protection**: Packets are verified against an RFC 6479 sliding sequence window, preventing any network attacker from capturing and replaying recorded speech.
