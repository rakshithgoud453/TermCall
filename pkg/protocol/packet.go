package protocol

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// Magic header bytes for TermCall packets ("TC")
const (
	MagicByte0 byte = 0x54 // 'T'
	MagicByte1 byte = 0x43 // 'C'
	Version1   byte = 0x01
	HeaderSize      = 16 // 2(magic) + 1(ver) + 1(type) + 8(seq) + 2(timestamp) + 2(length)
)

// PacketType defines the type of message transmitted in the datagram
type PacketType byte

const (
	TypeAudio     PacketType = 0x01 // Encrypted Opus audio frame
	TypeHandshake PacketType = 0x02 // Initial P2P punch / handshake
	TypeHandshakeAck PacketType = 0x03 // Handshake acknowledgement
	TypePing      PacketType = 0x04 // Latency / keepalive probe
	TypePong      PacketType = 0x05 // Latency response
	TypeMute      PacketType = 0x06 // Peer muted/unmuted notification
	TypeHangup    PacketType = 0x07 // Call teardown notice
)

var (
	ErrPacketTooShort = errors.New("packet data smaller than header size")
	ErrInvalidMagic   = errors.New("invalid protocol magic bytes")
	ErrInvalidVersion = errors.New("unsupported protocol version")
	ErrPayloadMismatch = errors.New("packet payload length mismatch")
)

// Packet represents a binary datagram in the TermCall wire protocol
type Packet struct {
	Version   byte
	Type      PacketType
	Seq       uint64 // Monotonic packet counter (anti-replay + AEAD nonce)
	Timestamp uint16 // 16-bit relative millisecond timestamp for jitter calculation
	Payload   []byte // Encrypted payload (or plaintext handshake during initial punch)
}

// Encode serializes the packet into wire format:
// [0..1]  Magic: 'T', 'C'
// [2]     Version: 0x01
// [3]     Type: PacketType
// [4..11] Sequence Number (uint64, Big Endian)
// [12..13] Timestamp (uint16, Big Endian)
// [14..15] Payload Length (uint16, Big Endian)
// [16..]  Payload bytes
func (p *Packet) Encode() []byte {
	payloadLen := len(p.Payload)
	buf := make([]byte, HeaderSize+payloadLen)

	buf[0] = MagicByte0
	buf[1] = MagicByte1
	buf[2] = p.Version
	if buf[2] == 0 {
		buf[2] = Version1
	}
	buf[3] = byte(p.Type)
	binary.BigEndian.PutUint64(buf[4:12], p.Seq)
	binary.BigEndian.PutUint16(buf[12:14], p.Timestamp)
	binary.BigEndian.PutUint16(buf[14:16], uint16(payloadLen))

	if payloadLen > 0 {
		copy(buf[HeaderSize:], p.Payload)
	}
	return buf
}

// Decode parses raw bytes into a Packet
func Decode(data []byte) (*Packet, error) {
	if len(data) < HeaderSize {
		return nil, ErrPacketTooShort
	}

	if data[0] != MagicByte0 || data[1] != MagicByte1 {
		return nil, ErrInvalidMagic
	}

	version := data[2]
	if version != Version1 {
		return nil, fmt.Errorf("%w: %d", ErrInvalidVersion, version)
	}

	pType := PacketType(data[3])
	seq := binary.BigEndian.Uint64(data[4:12])
	timestamp := binary.BigEndian.Uint16(data[12:14])
	payloadLen := int(binary.BigEndian.Uint16(data[14:16]))

	if len(data) < HeaderSize+payloadLen {
		return nil, fmt.Errorf("%w: expected %d, got %d", ErrPayloadMismatch, HeaderSize+payloadLen, len(data))
	}

	payload := make([]byte, payloadLen)
	if payloadLen > 0 {
		copy(payload, data[HeaderSize:HeaderSize+payloadLen])
	}

	return &Packet{
		Version:   version,
		Type:      pType,
		Seq:       seq,
		Timestamp: timestamp,
		Payload:   payload,
	}, nil
}
