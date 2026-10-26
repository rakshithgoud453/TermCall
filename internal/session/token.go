package session

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"strings"
)

const TokenPrefix = "tc_"

var (
	ErrInvalidTokenFormat = errors.New("token: invalid room token format")
	ErrInvalidKeyLength   = errors.New("token: invalid public key length")
)

// EncodeDualToken serializes a public STUN address, a local LAN address, and a 32-byte X25519 public key
// into an ICE-like room token (e.g. "tc_Ab39...").
// Format: [4 bytes Public IP] + [2 bytes Public Port] + [4 bytes LAN IP] + [2 bytes LAN Port] + [32 bytes PubKey] = 44 bytes
func EncodeDualToken(publicAddr, lanAddr *net.UDPAddr, pubKey []byte) (string, error) {
	if len(pubKey) != 32 {
		return "", fmt.Errorf("%w: expected 32, got %d", ErrInvalidKeyLength, len(pubKey))
	}

	pubIP := publicAddr.IP.To4()
	if pubIP == nil {
		pubIP = net.IPv4(127, 0, 0, 1)
	}

	lanIP := net.IPv4(127, 0, 0, 1)
	lanPort := publicAddr.Port
	if lanAddr != nil {
		if ip := lanAddr.IP.To4(); ip != nil {
			lanIP = ip
		}
		lanPort = lanAddr.Port
	}

	buf := make([]byte, 4+2+4+2+32)
	copy(buf[0:4], pubIP)
	buf[4] = byte(publicAddr.Port >> 8)
	buf[5] = byte(publicAddr.Port & 0xFF)

	copy(buf[6:10], lanIP)
	buf[10] = byte(lanPort >> 8)
	buf[11] = byte(lanPort & 0xFF)

	copy(buf[12:44], pubKey)

	encoded := base64.RawURLEncoding.EncodeToString(buf)
	return TokenPrefix + encoded, nil
}

// EncodeDirectToken serializes a single public address and public key (38 bytes)
func EncodeDirectToken(addr *net.UDPAddr, pubKey []byte) (string, error) {
	return EncodeDualToken(addr, nil, pubKey)
}

// DecodedToken holds the endpoints and public key extracted from a room token
type DecodedToken struct {
	PublicAddr *net.UDPAddr
	LANAddr    *net.UDPAddr
	PublicKey  []byte
}

// DecodeDirectToken deserializes a room token into candidate endpoints and X25519 public key
func DecodeDirectToken(token string) (*net.UDPAddr, []byte, error) {
	decoded, err := DecodeToken(token)
	if err != nil {
		return nil, nil, err
	}
	return decoded.PublicAddr, decoded.PublicKey, nil
}

// DecodeToken decodes either a 38-byte single-endpoint or 44-byte dual-endpoint room token
func DecodeToken(token string) (*DecodedToken, error) {
	token = strings.TrimSpace(token)
	if !strings.HasPrefix(token, TokenPrefix) {
		return nil, fmt.Errorf("%w: missing prefix '%s'", ErrInvalidTokenFormat, TokenPrefix)
	}

	payload := strings.TrimPrefix(token, TokenPrefix)
	raw, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return nil, fmt.Errorf("%w: base64 decode failed: %v", ErrInvalidTokenFormat, err)
	}

	// Case 1: Dual Endpoint (44 bytes)
	if len(raw) == 44 {
		pubIP := net.IPv4(raw[0], raw[1], raw[2], raw[3])
		pubPort := (int(raw[4]) << 8) | int(raw[5])

		lanIP := net.IPv4(raw[6], raw[7], raw[8], raw[9])
		lanPort := (int(raw[10]) << 8) | int(raw[11])

		pubKey := make([]byte, 32)
		copy(pubKey, raw[12:44])

		return &DecodedToken{
			PublicAddr: &net.UDPAddr{IP: pubIP, Port: pubPort},
			LANAddr:    &net.UDPAddr{IP: lanIP, Port: lanPort},
			PublicKey:  pubKey,
		}, nil
	}

	// Case 2: Single Endpoint (38 bytes)
	if len(raw) == 38 {
		ip := net.IPv4(raw[0], raw[1], raw[2], raw[3])
		port := (int(raw[4]) << 8) | int(raw[5])
		pubKey := make([]byte, 32)
		copy(pubKey, raw[6:38])

		addr := &net.UDPAddr{IP: ip, Port: port}
		return &DecodedToken{
			PublicAddr: addr,
			LANAddr:    addr,
			PublicKey:  pubKey,
		}, nil
	}

	return nil, fmt.Errorf("%w: unexpected payload length %d", ErrInvalidTokenFormat, len(raw))
}

// GetLocalLANIP returns the outbound LAN IP of this machine (e.g. 192.168.x.x)
func GetLocalLANIP() net.IP {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return net.IPv4(127, 0, 0, 1)
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP
}
