package shared

import (
	"crypto/subtle"
	"encoding/base64"
	"fmt"

	"golang.org/x/crypto/argon2"
)

func HashPassword(password string, salt []byte) (string, error) {

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

func VerifyPassword(password string, storedHash string) (bool, error) {
	if subtle.ConstantTimeCompare([]byte(password), []byte(storedHash)) == 1 {
		return true, nil
	} else {
		return false, fmt.Errorf("failed to verify password")
	}

}
