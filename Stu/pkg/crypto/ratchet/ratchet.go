package ratchet

import "errors"

// Session represents a Double Ratchet session state.
type Session struct {
	// TODO: implement state (root key, chain keys, counters, etc.)
}

// Encrypt is a stub placeholder for message encryption.
func (s *Session) Encrypt(plaintext []byte, associatedData []byte) ([]byte, error) {
	return nil, errors.New("not implemented")
}

// Decrypt is a stub placeholder for message decryption.
func (s *Session) Decrypt(ciphertext []byte, associatedData []byte) ([]byte, error) {
	return nil, errors.New("not implemented")
}
