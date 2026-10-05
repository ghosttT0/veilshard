package protocol

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
)

// Frame types
const (
	FrameTypeHandshake byte = 0x01
	FrameTypeData      byte = 0x02
	FrameTypePing      byte = 0x03
	FrameTypePong      byte = 0x04
	FrameTypeSwitch    byte = 0x05
	FrameTypeClose     byte = 0x06
)

// Frame flags
const (
	FlagNone       byte = 0x00
	FlagHasPadding byte = 0x01
	FlagStreamFIN  byte = 0x02
)

var (
	ErrFrameTooSmall = errors.New("frame too small")
	ErrInvalidLength = errors.New("invalid payload length")
)

// Frame represents an unencrypted protocol data unit inside the AEAD envelope.
type Frame struct {
	Type     byte
	StreamID uint32
	Flags    byte
	Data     []byte
	Padding  []byte
}

// Encode serializes the Frame into binary format:
// [Type: 1B] [StreamID: 4B] [Flags: 1B] [DataLen: 2B] [Data: N B] [PaddingLen: 2B] [Padding: M B]
func (f *Frame) Encode() []byte {
	dataLen := len(f.Data)
	paddingLen := len(f.Padding)

	totalLen := 1 + 4 + 1 + 2 + dataLen + 2 + paddingLen
	buf := make([]byte, totalLen)

	buf[0] = f.Type
	binary.BigEndian.PutUint32(buf[1:5], f.StreamID)
	buf[5] = f.Flags

	binary.BigEndian.PutUint16(buf[6:8], uint16(dataLen))
	if dataLen > 0 {
		copy(buf[8:8+dataLen], f.Data)
	}

	offset := 8 + dataLen
	binary.BigEndian.PutUint16(buf[offset:offset+2], uint16(paddingLen))
	if paddingLen > 0 {
		copy(buf[offset+2:], f.Padding)
	}

	return buf
}

// Decode deserializes binary bytes into a Frame.
func Decode(buf []byte) (*Frame, error) {
	if len(buf) < 10 { // 1 + 4 + 1 + 2 + 2 = 10 bytes minimum header
		return nil, ErrFrameTooSmall
	}

	f := &Frame{
		Type:     buf[0],
		StreamID: binary.BigEndian.Uint32(buf[1:5]),
		Flags:    buf[5],
	}

	dataLen := int(binary.BigEndian.Uint16(buf[6:8]))
	offset := 8
	if len(buf) < offset+dataLen+2 {
		return nil, ErrInvalidLength
	}

	if dataLen > 0 {
		f.Data = make([]byte, dataLen)
		copy(f.Data, buf[offset:offset+dataLen])
	}
	offset += dataLen

	paddingLen := int(binary.BigEndian.Uint16(buf[offset : offset+2]))
	offset += 2

	if len(buf) < offset+paddingLen {
		return nil, ErrInvalidLength
	}

	if paddingLen > 0 {
		f.Padding = make([]byte, paddingLen)
		copy(f.Padding, buf[offset:offset+paddingLen])
	}

	return f, nil
}

// NewDataFrame creates a data frame with optional morphing padding.
func NewDataFrame(streamID uint32, data []byte, paddingLen int, fin bool) *Frame {
	flags := FlagNone
	if fin {
		flags |= FlagStreamFIN
	}

	var pad []byte
	if paddingLen > 0 {
		flags |= FlagHasPadding
		pad = make([]byte, paddingLen)
		_, _ = rand.Read(pad)
	}

	return &Frame{
		Type:     FrameTypeData,
		StreamID: streamID,
		Flags:    flags,
		Data:     data,
		Padding:  pad,
	}
}

// NewPingFrame creates a lightweight keepalive or RTT measurement frame.
func NewPingFrame(seq uint32, paddingLen int) *Frame {
	payload := make([]byte, 8)
	binary.BigEndian.PutUint32(payload[0:4], seq)

	var pad []byte
	flags := FlagNone
	if paddingLen > 0 {
		flags |= FlagHasPadding
		pad = make([]byte, paddingLen)
		_, _ = rand.Read(pad)
	}

	return &Frame{
		Type:     FrameTypePing,
		StreamID: 0,
		Flags:    flags,
		Data:     payload,
		Padding:  pad,
	}
}
