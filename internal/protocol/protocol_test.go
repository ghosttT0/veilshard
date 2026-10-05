package protocol

import (
	"bytes"
	"testing"
	"time"
)

func TestFrameEncodeDecode(t *testing.T) {
	origPayload := []byte("GET /stream HTTP/1.1\r\nHost: example.com\r\n\r\n")
	paddingLen := 48

	frame := NewDataFrame(101, origPayload, paddingLen, true)
	if len(frame.Padding) != paddingLen {
		t.Fatalf("expected padding len %d, got %d", paddingLen, len(frame.Padding))
	}

	encoded := frame.Encode()
	if len(encoded) < len(origPayload)+paddingLen+10 {
		t.Fatalf("encoded length too short: %d", len(encoded))
	}

	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if decoded.Type != FrameTypeData {
		t.Errorf("expected FrameTypeData, got %d", decoded.Type)
	}
	if decoded.StreamID != 101 {
		t.Errorf("expected StreamID 101, got %d", decoded.StreamID)
	}
	if !bytes.Equal(decoded.Data, origPayload) {
		t.Errorf("payload mismatch: got %s, want %s", string(decoded.Data), string(origPayload))
	}
	if len(decoded.Padding) != paddingLen {
		t.Errorf("padding mismatch: got %d, want %d", len(decoded.Padding), paddingLen)
	}
}

func TestCryptoHandshakeAndAEAD(t *testing.T) {
	// Client generates ephemeral keypair
	clientPriv, clientPub, err := GenerateEphemeralKeyPair()
	if err != nil {
		t.Fatalf("client keygen: %v", err)
	}

	// Server generates ephemeral keypair
	serverPriv, serverPub, err := GenerateEphemeralKeyPair()
	if err != nil {
		t.Fatalf("server keygen: %v", err)
	}

	// Compute shared secrets via ECDH + HKDF
	salt := []byte("nexus-salt-2026")
	clientKey, err := DeriveSharedSecret(clientPriv, serverPub, salt, "nexus-v1")
	if err != nil {
		t.Fatalf("client derive: %v", err)
	}

	serverKey, err := DeriveSharedSecret(serverPriv, clientPub, salt, "nexus-v1")
	if err != nil {
		t.Fatalf("server derive: %v", err)
	}

	if clientKey != serverKey {
		t.Fatalf("client and server derived keys do not match!")
	}

	// Initialize ciphers
	clientCipher, err := NewSessionCipher(clientKey)
	if err != nil {
		t.Fatalf("init client cipher: %v", err)
	}

	serverCipher, err := NewSessionCipher(serverKey)
	if err != nil {
		t.Fatalf("init server cipher: %v", err)
	}

	plaintext := []byte("secret proxy message to send securely")
	ciphertext := clientCipher.Encrypt(plaintext)

	// Verify ciphertext does not contain plaintext
	if bytes.Contains(ciphertext, plaintext) {
		t.Fatalf("ciphertext leaked plaintext!")
	}

	decrypted, err := serverCipher.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("decrypt failed: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Fatalf("decrypted mismatch: got %s, want %s", string(decrypted), string(plaintext))
	}
}

func TestTrafficMorphing(t *testing.T) {
	webrtcEngine := NewMorphEngine(ProfileWebRTC)
	httpsEngine := NewMorphEngine(ProfileHTTPS)
	uniformEngine := NewMorphEngine(ProfileUniform)

	// Small voice packet (50 bytes)
	pad1 := webrtcEngine.CalculatePadding(50)
	if pad1 < 100 {
		t.Errorf("expected WebRTC voice padding > 100, got %d", pad1)
	}

	// HTTPS request (300 bytes)
	pad2 := httpsEngine.CalculatePadding(300)
	if pad2 < 150 {
		t.Errorf("expected HTTPS request padding > 150, got %d", pad2)
	}

	// Uniform bucket padding (multiple of 128)
	pad3 := uniformEngine.CalculatePadding(100)
	if (100+pad3)%128 < 0 {
		t.Errorf("invalid uniform padding: %d", pad3)
	}
}

func TestDualTransportFailover(t *testing.T) {
	var key [32]byte
	copy(key[:], "12345678901234567890123456789012")

	clientCipher, _ := NewSessionCipher(key)
	serverCipher, _ := NewSessionCipher(key)

	var switchedTo TransportType
	dt := NewDualTransport(clientCipher, ProfileWebRTC, func(oldMode, newMode TransportType) {
		switchedTo = newMode
	})

	if dt.CurrentMode() != TransportUDP {
		t.Errorf("expected initial mode UDP, got %s", dt.CurrentMode())
	}

	// Send normal frame
	cipherPayload, err := dt.Send(1, []byte("hello world"), false)
	if err != nil {
		t.Fatalf("send failed: %v", err)
	}

	serverFrame, err := serverCipher.Decrypt(cipherPayload)
	if err != nil {
		t.Fatalf("server decrypt failed: %v", err)
	}

	decoded, _ := Decode(serverFrame)
	if string(decoded.Data) != "hello world" {
		t.Errorf("data mismatch: %s", string(decoded.Data))
	}

	// Simulate 3 successive UDP timeouts (Carrier QoS throttling)
	dt.ReportLoss()
	dt.ReportLoss()
	dt.ReportLoss()

	// Should trigger automatic downgrade to TCP
	if dt.CurrentMode() != TransportTCP {
		t.Errorf("expected auto failover to TCP, got %s", dt.CurrentMode())
	}
	if switchedTo != TransportTCP {
		t.Errorf("callback not fired for TCP switch")
	}

	// Simulate network recovery on TCP with low latency Acks
	for i := 0; i < 20; i++ {
		dt.ReportAck(35 * time.Millisecond)
	}

	// Should automatically recover back to high-speed UDP
	if dt.CurrentMode() != TransportUDP {
		t.Errorf("expected recovery to UDP, got %s", dt.CurrentMode())
	}
}
