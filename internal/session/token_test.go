package session

import (
	"bytes"
	"net"
	"testing"
)

func TestDualTokenEncodeDecode(t *testing.T) {
	pubAddr := &net.UDPAddr{
		IP:   net.ParseIP("142.250.190.46"),
		Port: 51820,
	}
	lanAddr := &net.UDPAddr{
		IP:   net.ParseIP("192.168.1.105"),
		Port: 51820,
	}

	pubKey := make([]byte, 32)
	for i := range pubKey {
		pubKey[i] = byte(i * 7)
	}

	token, err := EncodeDualToken(pubAddr, lanAddr, pubKey)
	if err != nil {
		t.Fatalf("failed to encode dual token: %v", err)
	}

	t.Logf("Generated dual token: %s", token)

	decoded, err := DecodeToken(token)
	if err != nil {
		t.Fatalf("failed to decode token: %v", err)
	}

	if !decoded.PublicAddr.IP.Equal(pubAddr.IP) || decoded.PublicAddr.Port != pubAddr.Port {
		t.Errorf("public addr mismatch: got %v, want %v", decoded.PublicAddr, pubAddr)
	}
	if !decoded.LANAddr.IP.Equal(lanAddr.IP) || decoded.LANAddr.Port != lanAddr.Port {
		t.Errorf("lan addr mismatch: got %v, want %v", decoded.LANAddr, lanAddr)
	}
	if !bytes.Equal(decoded.PublicKey, pubKey) {
		t.Errorf("pubKey mismatch: got %x, want %x", decoded.PublicKey, pubKey)
	}
}
