package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"

	"golang.org/x/crypto/hkdf"

	"github.com/JohnnyGlynn/strike/internal/client/types"
)

func DeriveKeys(c *types.ClientInfo, sct []byte) error {

	const keyLen = 32 //256 bits

	d := hkdf.New(sha256.New, sct, nil, nil)

	encKey := make([]byte, keyLen)
	hmacKey := make([]byte, keyLen)

	if _, err := io.ReadFull(d, encKey); err != nil {
		return err
	}

	c.Cache.ActiveChat.EncKey = encKey

	if _, err := io.ReadFull(d, hmacKey); err != nil {
		return err
	}

	c.Cache.ActiveChat.HmacKey = hmacKey

	return nil
}

func VerifyEdSignatures(pubKey ed25519.PublicKey, nonce, CurvePublicKey []byte, sigs [][]byte) bool {
	if len(sigs) < 2 {
		return false
	}

	if !ed25519.Verify(pubKey, nonce, sigs[0]) {
		return false
	}

	return ed25519.Verify(pubKey, CurvePublicKey, sigs[1])
}

func Encrypt(c *types.ClientInfo, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(c.Cache.ActiveChat.EncKey)
	if err != nil {
		return nil, err
	}

	//Galois/Counter Mode - https://en.wikipedia.org/wiki/Galois/Counter_Mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	sealedMessage := append(nonce, ciphertext...)

	return sealedMessage, nil
}

func Decrypt(c *types.ClientInfo, sealedMessage []byte) ([]byte, error) {
	block, err := aes.NewCipher(c.Cache.ActiveChat.EncKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(sealedMessage) < nonceSize {
		return nil, fmt.Errorf("data too short")
	}

	nonce := sealedMessage[:nonceSize]
	ciphertext := sealedMessage[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}
