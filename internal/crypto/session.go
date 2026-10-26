package crypto

import (
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/hkdf"
)

var (
	ErrReplayDetected     = errors.New("crypto: packet replay detected")
	ErrAuthenticationFail = errors.New("crypto: chacha20poly1305 authentication failed")
	ErrKeyExchangeFailed  = errors.New("crypto: ecdh key exchange failed")
)

// KeyPair represents an ephemeral X25519 key pair
type KeyPair struct {
	PrivateKey *ecdh.PrivateKey
	PublicKey  *ecdh.PublicKey
}

// GenerateKeyPair creates a fresh ephemeral X25519 keypair in RAM
func GenerateKeyPair() (*KeyPair, error) {
	curve := ecdh.X25519()
	priv, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate X25519 key: %w", err)
	}
	return &KeyPair{
		PrivateKey: priv,
		PublicKey:  priv.PublicKey(),
	}, nil
}

// PublicKeyBytes returns the 32-byte raw public key
func (kp *KeyPair) PublicKeyBytes() []byte {
	return kp.PublicKey.Bytes()
}

// Role defines the participant role in key derivation
type Role int

const (
	RoleHost Role = 1
	RolePeer Role = 2
)

// CryptoSession handles symmetric encryption, decryption, and replay prevention
type CryptoSession struct {
	role         Role
	sharedSecret []byte
	txAead       cipher.AEAD
	rxAead       cipher.AEAD
	replayWindow *ReplayWindow
}

// NewCryptoSession establishes a cryptographically secure session given the local keypair,
// remote public key bytes, and local role (Host or Peer).
func NewCryptoSession(localKeyPair *KeyPair, remotePubKeyBytes []byte, role Role) (*CryptoSession, error) {
	curve := ecdh.X25519()
	remotePub, err := curve.NewPublicKey(remotePubKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid remote public key: %v", ErrKeyExchangeFailed, err)
	}

	sharedSecret, err := localKeyPair.PrivateKey.ECDH(remotePub)
	if err != nil {
		return nil, fmt.Errorf("%w: ecdh computation failed: %v", ErrKeyExchangeFailed, err)
	}

	// Derive directional 256-bit keys via HKDF-SHA256
	// Info "termcall-host-to-peer" and "termcall-peer-to-host"
	hostToPeerKey := make([]byte, chacha20poly1305.KeySize)
	peerToHostKey := make([]byte, chacha20poly1305.KeySize)

	salt := []byte("termcall-v1-hkdf-salt")
	hkdfHost := hkdf.New(sha256.New, sharedSecret, salt, []byte("termcall-host-to-peer"))
	if _, err := io.ReadFull(hkdfHost, hostToPeerKey); err != nil {
		return nil, err
	}

	hkdfPeer := hkdf.New(sha256.New, sharedSecret, salt, []byte("termcall-peer-to-host"))
	if _, err := io.ReadFull(hkdfPeer, peerToHostKey); err != nil {
		return nil, err
	}

	var txKey, rxKey []byte
	if role == RoleHost {
		txKey = hostToPeerKey
		rxKey = peerToHostKey
	} else {
		txKey = peerToHostKey
		rxKey = hostToPeerKey
	}

	txAead, err := chacha20poly1305.New(txKey)
	if err != nil {
		return nil, err
	}

	rxAead, err := chacha20poly1305.New(rxKey)
	if err != nil {
		return nil, err
	}

	return &CryptoSession{
		role:         role,
		sharedSecret: sharedSecret,
		txAead:       txAead,
		rxAead:       rxAead,
		replayWindow: NewReplayWindow(128),
	}, nil
}

// buildNonce formats a 12-byte ChaCha20-Poly1305 nonce from a 64-bit sequence number
func buildNonce(seq uint64, role Role) []byte {
	nonce := make([]byte, chacha20poly1305.NonceSize) // 12 bytes
	nonce[0] = byte(role)
	binary.BigEndian.PutUint64(nonce[4:12], seq)
	return nonce
}

// Encrypt seals plaintext audio data with AEAD using the monotonic sequence number
func (s *CryptoSession) Encrypt(seq uint64, plaintext []byte, additionalData []byte) []byte {
	nonce := buildNonce(seq, s.role)
	return s.txAead.Seal(nil, nonce, plaintext, additionalData)
}

// Decrypt opens and authenticates ciphertext audio data and checks replay protection
func (s *CryptoSession) Decrypt(seq uint64, ciphertext []byte, additionalData []byte) ([]byte, error) {
	// 1. Anti-replay check
	if !s.replayWindow.CheckAndAccept(seq) {
		return nil, ErrReplayDetected
	}

	// 2. Derive remote peer's role for nonce construction
	var remoteRole Role
	if s.role == RoleHost {
		remoteRole = RolePeer
	} else {
		remoteRole = RoleHost
	}

	nonce := buildNonce(seq, remoteRole)

	// 3. Authenticated decryption
	plaintext, err := s.rxAead.Open(nil, nonce, ciphertext, additionalData)
	if err != nil {
		return nil, ErrAuthenticationFail
	}

	return plaintext, nil
}

// Close zeroes out sensitive key material from memory
func (s *CryptoSession) Close() {
	if s.sharedSecret != nil {
		for i := range s.sharedSecret {
			s.sharedSecret[i] = 0
		}
		s.sharedSecret = nil
	}
}
