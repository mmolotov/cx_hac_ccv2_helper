package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/crypto/argon2"
)

const (
	vaultFileName = "credentials.enc"

	// File layout: [4B magic][16B salt][12B nonce][ciphertext+16B GCM tag]
	magicSize = 4
	saltSize  = 16
	nonceSize = 12

	argon2Time    = 1
	argon2Memory  = 64 * 1024 // 64 MiB
	argon2Threads = 4
	argon2KeyLen  = 32
)

var magic = [magicSize]byte{'H', 'A', 'C', 'V'}

// vaultPath returns the path to credentials.enc.
func vaultPath() (string, error) {
	dir, err := ConfigDirPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, vaultFileName), nil
}

// deriveKey derives a 32-byte AES key from masterPassword and salt using Argon2id.
func deriveKey(masterPassword string, salt []byte) []byte {
	return argon2.IDKey(
		[]byte(masterPassword),
		salt,
		argon2Time,
		argon2Memory,
		argon2Threads,
		argon2KeyLen,
	)
}

// SaveCredentials encrypts store with masterPassword and writes the vault file.
func SaveCredentials(store *CredentialsStore, masterPassword string) error {
	path, err := vaultPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating vault dir: %w", err)
	}

	plaintext, err := json.Marshal(store)
	if err != nil {
		return fmt.Errorf("marshalling credentials: %w", err)
	}

	var salt [saltSize]byte
	if _, err := io.ReadFull(rand.Reader, salt[:]); err != nil {
		return fmt.Errorf("generating salt: %w", err)
	}

	key := deriveKey(masterPassword, salt[:])

	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("creating cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("creating GCM: %w", err)
	}

	var nonce [nonceSize]byte
	if _, err := io.ReadFull(rand.Reader, nonce[:]); err != nil {
		return fmt.Errorf("generating nonce: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce[:], plaintext, nil)

	// Build file: [4B magic][16B salt][12B nonce][ciphertext]
	buf := make([]byte, 0, magicSize+saltSize+nonceSize+len(ciphertext))
	buf = append(buf, magic[:]...)
	buf = append(buf, salt[:]...)
	buf = append(buf, nonce[:]...)
	buf = append(buf, ciphertext...)

	if err := os.WriteFile(path, buf, 0o600); err != nil {
		return fmt.Errorf("writing vault file: %w", err)
	}
	return nil
}

// LoadCredentials decrypts and returns the CredentialsStore.
// Returns an empty store (no error) if the vault file does not exist.
func LoadCredentials(masterPassword string) (*CredentialsStore, error) {
	path, err := vaultPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &CredentialsStore{Projects: make(map[string]ProjectCreds)}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading vault file: %w", err)
	}

	minSize := magicSize + saltSize + nonceSize
	if len(data) < minSize {
		return nil, errors.New("vault file too small")
	}

	// Parse header
	var fileMagic [magicSize]byte
	copy(fileMagic[:], data[:magicSize])
	if fileMagic != magic {
		return nil, errors.New("invalid vault magic bytes")
	}

	offset := magicSize
	salt := data[offset : offset+saltSize]
	offset += saltSize
	nonce := data[offset : offset+nonceSize]
	offset += nonceSize
	ciphertext := data[offset:]

	key := deriveKey(masterPassword, salt)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("creating cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("creating GCM: %w", err)
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypting vault (wrong master password?): %w", err)
	}

	var store CredentialsStore
	if err := json.Unmarshal(plaintext, &store); err != nil {
		return nil, fmt.Errorf("unmarshalling credentials: %w", err)
	}
	if store.Projects == nil {
		store.Projects = make(map[string]ProjectCreds)
	}
	return &store, nil
}

// ensure binary import used (for magic bytes encoding in tests)
var _ = binary.BigEndian
