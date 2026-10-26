# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 0.1.x   | :white_check_mark: |

## Security Invariants

TermCall is built with zero-telemetry and local-first cryptographic principles:

1. **Ephemeral Key Material**:
   - Every call session generates fresh 256-bit X25519 ECDH keypairs purely in physical RAM (`GenerateKeyPair()`).
   - Private keys never touch non-volatile disk storage and are actively zeroed out upon session termination (`Close()`).

2. **Directional AEAD Keys**:
   - HKDF-SHA256 derives two isolated symmetric keys (`termcall-host-to-peer` and `termcall-peer-to-host`) to ensure immunity against packet reflection attacks.
   - All audio datagrams are authenticated and encrypted with ChaCha20-Poly1305.

3. **Replay Attack Mitigation**:
   - Monotonic 64-bit sequence counters act as nonces.
   - Incoming packets are validated through an RFC 6479 128-packet sliding bitmap (`ReplayWindow`). Duplicate, out-of-window, or excessively delayed datagrams are immediately rejected.

4. **Zero Intermediary Decryption**:
   - Signaling uses serverless dual-candidate room tokens (`tc_<base64>`).
   - UDP datagrams are exchanged directly peer-to-peer. No intermediate relays or cloud proxies ever possess symmetric encryption keys or decrypt audio frames.

## Reporting a Vulnerability

If you discover a security vulnerability or cryptographic flaw in TermCall, please report it responsibly:

- **Email**: Send vulnerability reports directly to **[rakshithgoud453@gmail.com](mailto:rakshithgoud453@gmail.com)**.
- **Details**: Please include a proof of concept or steps to reproduce, the impacted version, and your assessment of the vulnerability's impact.
- **Response**: We take security issues seriously and will acknowledge receipt of your report within 48 hours and coordinate a fix prior to public disclosure.
