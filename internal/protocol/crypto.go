package protocol

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"sync/atomic"
)

var (
	ErrDecryptionFailed = errors.New("aead decryption or authentication failed")
	ErrReplayDetected   = errors.New("potential replay attack detected")
)

// SessionCipher manages symmetric AEAD encryption with monotonic nonces.
type SessionCipher struct {
	aead       cipher.AEAD
	sendNonce  uint64
	recvNonce  uint64
	key        [32]byte
}

// NewSessionCipher initializes AES-256-GCM cipher with a 32-byte shared secret.
func NewSessionCipher(key [32]byte) (*SessionCipher, error) {
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("aes new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("cipher new gcm: %w", err)
	}

	return &SessionCipher{
		aead: gcm,
		key:  key,
	}, nil
}

// Encrypt seals plaintext with monotonic nonce.
// Returns: [Ciphertext + 16B Tag]
func (sc *SessionCipher) Encrypt(plaintext []byte) []byte {
	nonceVal := atomic.AddUint64(&sc.sendNonce, 1)

	var nonce [12]byte
	binary.BigEndian.PutUint64(nonce[4:12], nonceVal)

	// Seal appends ciphertext + 16-byte authentication tag
	ciphertext := sc.aead.Seal(nil, nonce[:], plaintext, nil)
	return ciphertext
}

// Decrypt opens ciphertext with the expected next nonce.
func (sc *SessionCipher) Decrypt(ciphertext []byte) ([]byte, error) {
	nonceVal := atomic.AddUint64(&sc.recvNonce, 1)

	var nonce [12]byte
	binary.BigEndian.PutUint64(nonce[4:12], nonceVal)

	plaintext, err := sc.aead.Open(nil, nonce[:], ciphertext, nil)
	if err != nil {
		return nil, ErrDecryptionFailed
	}

	return plaintext, nil
}

// DeriveSharedSecret performs X25519 ECDH key agreement and expands to a 32-byte symmetric key via HKDF.
func DeriveSharedSecret(priv *ecdh.PrivateKey, peerPub *ecdh.PublicKey, salt []byte, info string) ([32]byte, error) {
	var key [32]byte

	rawShared, err := priv.ECDH(peerPub)
	if err != nil {
		return key, fmt.Errorf("ecdh compute shared: %w", err)
	}

	// HKDF-Extract & Expand
	derived := hkdf(rawShared, salt, []byte(info), 32)
	copy(key[:], derived)

	return key, nil
}

// Standard HKDF implementation using SHA-256
func hkdf(ikm, salt, info []byte, length int) []byte {
	if len(salt) == 0 {
		salt = make([]byte, 32)
	}

	// 1. Extract: PRK = HMAC-Hash(salt, IKM)
	h := hmac.New(sha256.New, salt)
	h.Write(ikm)
	prk := h.Sum(nil)

	// 2. Expand: OKM
	var okm []byte
	var t []byte
	counter := byte(1)

	for len(okm) < length {
		hExp := hmac.New(sha256.New, prk)
		hExp.Write(t)
		hExp.Write(info)
		hExp.Write([]byte{counter})
		t = hExp.Sum(nil)
		okm = append(okm, t...)
		counter++
	}

	return okm[:length]
}

// GenerateEphemeralKeyPair creates a fresh Curve25519 keypair for 0-RTT/1-RTT key exchange.
func GenerateEphemeralKeyPair() (*ecdh.PrivateKey, *ecdh.PublicKey, error) {
	priv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	return priv, priv.PublicKey(), nil
}
