package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"io"

	"golang.org/x/crypto/argon2"
)

const (
	keyLength    = 32
	saltLength   = 16
	nonceLength  = 12
	argon2Time   = 2
	argon2Memory = 19456
	argon2Thread = 1
)

// generateSalt creates a new random salt for key derivation.
// Returns the generated salt or an error if random generation fails.
func generateSalt() ([]byte, error) {
	salt := make([]byte, saltLength)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}
	return salt, nil
}

func generateNonce() ([]byte, error) {
	nonce := make([]byte, nonceLength)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return nonce, nil
}

// generateKey creates a new random key for encryption.
// Returns the generated key or an error if random generation fails.
func generateKey() ([]byte, error) {
	key := make([]byte, keyLength)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, err
	}
	return key, nil
}

// deriveKey creates a new key for encryption using Argon2.
// Returns the derived key or an error if key derivation fails.
func deriveKey(password string, salt []byte) []byte {
	return argon2.IDKey([]byte(password), salt, argon2Time, argon2Memory, argon2Thread, keyLength)
}

// encrypt encrypts the plaintext with the given key.
// Returns the ciphertext, nonce, and an error if encryption fails.
func encrypt(plaintext, key []byte) (ciphertext, nonce []byte, err error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}
	nonce, err = generateNonce()
	if err != nil {
		return nil, nil, err
	}
	ciphertext = gcm.Seal(nil, nonce, plaintext, nil)
	return ciphertext, nonce, nil
}

// decrypt decrypts the ciphertext with the given key and nonce.
// Returns the plaintext as a byte slice or an error if decryption fails.
func decrypt(ciphertext, key, nonce []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}
	return plaintext, nil
}

// encryptSecretValue encrypts the plaintext secret value using the vault key and nonce.
// Returns the encrypted secret value or an error if encryption fails.
func encryptSecretValue(plaintext, vaultKey, nonce []byte) ([]byte, error) {
	block, err := aes.NewCipher(vaultKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return gcm.Seal(nil, nonce, plaintext, nil), nil
}

// decryptSecretValue decrypts the ciphertext secret value using the vault key and nonce.
// Returns the decrypted secret value or an error if decryption fails.
func decryptSecretValue(ciphertext, vaultKey, nonce []byte) ([]byte, error) {
	block, err := aes.NewCipher(vaultKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return gcm.Open(nil, nonce, ciphertext, nil)
}
