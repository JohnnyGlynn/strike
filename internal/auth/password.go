package password

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

func HashPassword(password string) (string, error) {
	salt := make([]byte, 16)

	//add salt
	_, err := rand.Read(salt)
	if err != nil {
		return "", fmt.Errorf("failed to generate salt: %w", err)
	}

	// Hash Params
	time := uint32(2)
	memory := uint32(128 * 1024) //(128 MiB)
	threads := uint8(4)
	keyLen := uint32(32)
	// argon2.Version//19

	hash := argon2.IDKey([]byte(password), salt, time, memory, threads, keyLen)

	// Encode salt and hash into a single string
	saltEncoded := base64.RawStdEncoding.EncodeToString(salt)
	hashEncoded := base64.RawStdEncoding.EncodeToString(hash)

	return fmt.Sprintf("argon2id$v=19$%s$%s", saltEncoded, hashEncoded), nil
}

func VerifyPassword(password, storedHash string) (bool, error) {
	argonHash := strings.Split(storedHash, "$")
	if len(argonHash) != 4 {
		return false, fmt.Errorf("invalid hash format")
	}

	//check for argon2 and current version (19)
	if argonHash[0] != "argon2id" || argonHash[1] != "v=19" {
		return false, fmt.Errorf("incorrect hash version")
	}

	// Salt decode
	salt, err := base64.RawStdEncoding.DecodeString(argonHash[2])
	if err != nil {
		return false, fmt.Errorf("failed to decode salt: %w", err)
	}

	// Hash decode
	hash, err := base64.RawStdEncoding.DecodeString(argonHash[3])
	if err != nil {
		return false, fmt.Errorf("failed to decode hash: %w", err)
	}

	// Hash Params
	time := uint32(2)
	memory := uint32(128 * 1024) //(128 MiB)
	threads := uint8(4)
	keyLen := uint32(32)

	// Recompute the hash with the provided password and salt
	checkHash := argon2.IDKey([]byte(password), salt, time, memory, threads, keyLen)

	//Compare checkHash with stored password hash
	return subtle.ConstantTimeCompare(checkHash, hash) == 1, nil
}
