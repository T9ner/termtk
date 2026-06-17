package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha512"
	"encoding/base64"
	"fmt"

	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/nacl/box"
)

// Ed25519ToX25519Private converts an Ed25519 private key to an X25519 private key.
// Uses SHA-512 of the seed with standard clamping per RFC 7748.
func Ed25519ToX25519Private(edPriv ed25519.PrivateKey) [32]byte {
	h := sha512.Sum512(edPriv.Seed())
	var x25519Priv [32]byte
	copy(x25519Priv[:], h[:32])
	// Clamp per RFC 7748
	x25519Priv[0] &= 248
	x25519Priv[31] &= 127
	x25519Priv[31] |= 64
	return x25519Priv
}

// Ed25519ToX25519Public derives the X25519 public key from an Ed25519 private key.
// We derive through the private key because direct Ed25519→X25519 public key
// conversion requires edwards25519 point decompression which isn't in stdlib.
func Ed25519ToX25519Public(edPriv ed25519.PrivateKey) ([32]byte, error) {
	x25519Priv := Ed25519ToX25519Private(edPriv)
	result, err := curve25519.X25519(x25519Priv[:], curve25519.Basepoint)
	if err != nil {
		return [32]byte{}, fmt.Errorf("failed to derive X25519 public key: %w", err)
	}
	var pub [32]byte
	copy(pub[:], result)
	return pub, nil
}

// Encrypt encrypts plaintext using NaCl box (Curve25519-XSalsa20-Poly1305) with a random nonce.
// Returns base64-encoded ciphertext and base64-encoded nonce.
func Encrypt(plaintext []byte, senderEdPriv ed25519.PrivateKey, recipientX25519Pub [32]byte) (ciphertext, nonce string, err error) {
	var n [24]byte
	if _, err := rand.Read(n[:]); err != nil {
		return "", "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	senderX25519Priv := Ed25519ToX25519Private(senderEdPriv)

	encrypted := box.Seal(nil, plaintext, &n, &recipientX25519Pub, &senderX25519Priv)

	return base64.StdEncoding.EncodeToString(encrypted),
		base64.StdEncoding.EncodeToString(n[:]),
		nil
}

// Decrypt decrypts a NaCl box ciphertext.
// Expects base64-encoded ciphertext and nonce.
func Decrypt(ciphertextB64, nonceB64 string, recipientEdPriv ed25519.PrivateKey, senderX25519Pub [32]byte) ([]byte, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(ciphertextB64)
	if err != nil {
		return nil, fmt.Errorf("invalid ciphertext base64: %w", err)
	}
	nonceBytes, err := base64.StdEncoding.DecodeString(nonceB64)
	if err != nil {
		return nil, fmt.Errorf("invalid nonce base64: %w", err)
	}
	if len(nonceBytes) != 24 {
		return nil, fmt.Errorf("invalid nonce length: %d", len(nonceBytes))
	}

	var nonce [24]byte
	copy(nonce[:], nonceBytes)

	recipientX25519Priv := Ed25519ToX25519Private(recipientEdPriv)

	plaintext, ok := box.Open(nil, ciphertext, &nonce, &senderX25519Pub, &recipientX25519Priv)
	if !ok {
		return nil, fmt.Errorf("decryption failed (authentication error)")
	}
	return plaintext, nil
}
