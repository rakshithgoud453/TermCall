package crypto

import (
	"bytes"
	"testing"
)

func TestCryptoSessionEndToEnd(t *testing.T) {
	// Generate keypairs for Host and Peer
	hostKeys, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate host keys: %v", err)
	}

	peerKeys, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate peer keys: %v", err)
	}

	// Host establishes crypto session with peer's public key
	hostSession, err := NewCryptoSession(hostKeys, peerKeys.PublicKeyBytes(), RoleHost)
	if err != nil {
		t.Fatalf("failed to create host session: %v", err)
	}
	defer hostSession.Close()

	// Peer establishes crypto session with host's public key
	peerSession, err := NewCryptoSession(peerKeys, hostKeys.PublicKeyBytes(), RolePeer)
	if err != nil {
		t.Fatalf("failed to create peer session: %v", err)
	}
	defer peerSession.Close()

	// 1. Host encrypts audio packet, Peer decrypts
	plaintext := []byte("audio-sample-data-48khz-frame")
	aad := []byte{0x54, 0x43, 0x01, 0x01} // Magic and type
	seq := uint64(1)

	ciphertext := hostSession.Encrypt(seq, plaintext, aad)
	decrypted, err := peerSession.Decrypt(seq, ciphertext, aad)
	if err != nil {
		t.Fatalf("peer failed to decrypt host message: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("decrypted content mismatch: got %s, want %s", string(decrypted), string(plaintext))
	}

	// 2. Peer encrypts audio packet, Host decrypts
	peerPlaintext := []byte("peer-reply-audio-frame")
	peerCiphertext := peerSession.Encrypt(seq, peerPlaintext, aad)
	hostDecrypted, err := hostSession.Decrypt(seq, peerCiphertext, aad)
	if err != nil {
		t.Fatalf("host failed to decrypt peer message: %v", err)
	}

	if !bytes.Equal(hostDecrypted, peerPlaintext) {
		t.Errorf("host decrypted content mismatch: got %s, want %s", string(hostDecrypted), string(peerPlaintext))
	}

	// 3. Test Replay Detection (attempting to decrypt same sequence again)
	_, err = peerSession.Decrypt(seq, ciphertext, aad)
	if err != ErrReplayDetected {
		t.Errorf("expected ErrReplayDetected, got %v", err)
	}

	// 4. Test Tamper / Authentication Failure
	tamperedCiphertext := make([]byte, len(ciphertext))
	copy(tamperedCiphertext, ciphertext)
	tamperedCiphertext[len(tamperedCiphertext)-1] ^= 0xFF // Corrupt MAC tag

	_, err = peerSession.Decrypt(uint64(2), tamperedCiphertext, aad)
	if err != ErrAuthenticationFail {
		t.Errorf("expected ErrAuthenticationFail, got %v", err)
	}
}

func TestReplayWindow(t *testing.T) {
	rw := NewReplayWindow(64)

	// First packet: seq 100
	if !rw.CheckAndAccept(100) {
		t.Fatal("expected seq 100 to be accepted")
	}

	// Immediate replay
	if rw.CheckAndAccept(100) {
		t.Fatal("expected seq 100 to be rejected as duplicate")
	}

	// Newer packet: seq 102
	if !rw.CheckAndAccept(102) {
		t.Fatal("expected seq 102 to be accepted")
	}

	// Out of order packet within window: seq 101
	if !rw.CheckAndAccept(101) {
		t.Fatal("expected out of order seq 101 to be accepted")
	}

	// Duplicate of 101
	if rw.CheckAndAccept(101) {
		t.Fatal("expected duplicate seq 101 to be rejected")
	}

	// Duplicate of 100 after shift
	if rw.CheckAndAccept(100) {
		t.Fatal("expected duplicate seq 100 to be rejected")
	}

	// Advance window far ahead: seq 200 (diff = 98 > window 64)
	if !rw.CheckAndAccept(200) {
		t.Fatal("expected seq 200 to be accepted")
	}

	// Ancient packet (seq 102 is now diff 98 > 64)
	if rw.CheckAndAccept(102) {
		t.Fatal("expected ancient seq 102 to be rejected")
	}
}
