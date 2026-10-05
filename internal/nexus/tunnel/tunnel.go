package tunnel

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"

	"github.com/veilshard/veilshard/internal/protocol"
)

// WriteFrame wraps data into an authenticated protocol frame with morphing padding,
// encrypts it, and writes length-prefixed ciphertext to conn.
func WriteFrame(conn net.Conn, cipher *protocol.SessionCipher, morph *protocol.MorphEngine, streamID uint32, data []byte, fin bool) error {
	paddingLen := 0
	if morph != nil {
		paddingLen = morph.CalculatePadding(len(data))
	}

	frame := protocol.NewDataFrame(streamID, data, paddingLen, fin)
	rawBytes := frame.Encode()

	ciphertext := cipher.Encrypt(rawBytes)
	cipherLen := len(ciphertext)

	if cipherLen > 65535 {
		return fmt.Errorf("ciphertext length %d exceeds max frame size", cipherLen)
	}

	// Wire: [2B length] [Ciphertext]
	packet := make([]byte, 2+cipherLen)
	binary.BigEndian.PutUint16(packet[0:2], uint16(cipherLen))
	copy(packet[2:], ciphertext)

	_, err := conn.Write(packet)
	return err
}

// ReadFrame reads a length-prefixed packet from conn, decrypts it, and returns the protocol Frame.
func ReadFrame(conn net.Conn, cipher *protocol.SessionCipher) (*protocol.Frame, error) {
	lenBuf := make([]byte, 2)
	if _, err := io.ReadFull(conn, lenBuf); err != nil {
		return nil, err
	}

	cipherLen := int(binary.BigEndian.Uint16(lenBuf))
	if cipherLen <= 0 {
		return nil, fmt.Errorf("invalid frame length %d", cipherLen)
	}

	cipherBuf := make([]byte, cipherLen)
	if _, err := io.ReadFull(conn, cipherBuf); err != nil {
		return nil, err
	}

	plaintext, err := cipher.Decrypt(cipherBuf)
	if err != nil {
		return nil, fmt.Errorf("decrypt error: %w", err)
	}

	return protocol.Decode(plaintext)
}
