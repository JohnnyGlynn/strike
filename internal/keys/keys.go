package keys

import (
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func SigningKeygen() error {

	//TODO: There is definetly a better way to do this
	fmt.Println("WARNING: You (the user) are responsible for the safety of these key files. You will not be able to recover these files if they are lost")

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("error Generating Signing keys: %v", err)
	}

	// Encode PKCS#8 format
	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return fmt.Errorf("error encoding private key: %v", err)
	}

	err = writeToPem(privateKeyBytes, "PRIVATE KEY", "strike_signing.pem")
	if err != nil {
		return fmt.Errorf("failed to write private key: %v", err)
	}

	// Encode PKIX format
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return fmt.Errorf("error encoding public key: %v", err)
	}

	err = writeToPem(publicKeyBytes, "PUBLIC KEY", "strike_public_signing.pem")
	if err != nil {
		return fmt.Errorf("failed to write public key: %v", err)
	}

	fmt.Println("Strike Signing Keys generated and saved to ~/.strike-keys")
	return nil

}

func ValidateSigningKeys(keyBytes []byte) error {
	// Decode PEM.
	block, _ := pem.Decode(keyBytes)
	if block == nil {
		return fmt.Errorf("failed to decode PEM block")
	}

	// Check for Private or Public Key
	switch block.Type {

	case "PRIVATE KEY":
		//Check key type
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return fmt.Errorf("failed to parse private key: %w", err)
		}
		//ok if ed25519
		_, ok := key.(ed25519.PrivateKey)
		if !ok {
			return fmt.Errorf("invalid ED25519 private key")
		}
		fmt.Println("Valid ED25519 private key detected.")

	case "PUBLIC KEY":
		key, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return fmt.Errorf("failed to parse public key: %w", err)
		}
		_, ok := key.(ed25519.PublicKey)
		if !ok {
			return fmt.Errorf("invalid ED25519 public key")
		}
		fmt.Println("Valid ED25519 public key detected.")

	default:
		return fmt.Errorf("unsupported key type: %s", block.Type)
	}

	return nil
}

func EncryptionKeygen() error {
	curve := ecdh.X25519()

	privateKey, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("error Generating encryption key: %v", err)
	}

	// Extract the private and public keys as byte slices
	privateKeyBytes := privateKey.Bytes()
	err = writeToPem(privateKeyBytes, "PRIVATE KEY", "strike_encryption.pem")
	if err != nil {
		return fmt.Errorf("failed to write private key: %v", err)
	}

	publicKeyBytes := privateKey.PublicKey().Bytes()
	err = writeToPem(publicKeyBytes, "PUBLIC KEY", "strike_public_encryption.pem")
	if err != nil {
		return fmt.Errorf("failed to write private key: %v", err)
	}

	fmt.Println("Strike Encryption Keys generated and saved to ~/.strike-keys")
	return nil

}

func ValidateEncryptionKeys(keyBytes []byte) error {
	// Decode PEM
	block, _ := pem.Decode(keyBytes)
	if block == nil {
		return fmt.Errorf("failed to decode PEM block")
	}

	// Check for Private or Public Key
	switch block.Type {
	case "PRIVATE KEY":
		curve := ecdh.X25519()
		privateKey, err := curve.NewPrivateKey(block.Bytes)
		if err == nil {
			// Derive public key for validity
			publicKey := privateKey.PublicKey()
			if len(publicKey.Bytes()) == 32 {
				fmt.Println("Valid Curve25519 private key detected.")
				return nil
			}
		}

		return fmt.Errorf("invalid Curve25519 private key")

	case "PUBLIC KEY":
		// Curve25519 (32 bytes - raw public key)
		if len(block.Bytes) == 32 {
			curve := ecdh.X25519()
			_, err := curve.NewPublicKey(block.Bytes)
			if err != nil {
				return fmt.Errorf("invalid Curve25519 private key")
			}
		}

		fmt.Println("Valid Curve25519 public key detected.")

	}

	return nil
}

func GetKeyFromPath(path string) ([]byte, error) {

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	//TODO: Remove this hacky way of ensuring that ~ is handled if provided in config
	if strings.HasPrefix(path, "~") {
		path = filepath.Join(homeDir, path[1:])
	}

	keyFile, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("error opening key file: %v", err)
	}
	defer keyFile.Close()

	key, err := io.ReadAll(keyFile)
	if err != nil {
		return nil, fmt.Errorf("error reading key file: %v", err)
	}

	return key, nil
}

func writeToPem(keyBytes []byte, keyType string, keyNameDotPem string) error {

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("error finding user home directory: %v", err)
	}

	strikeKeyDir := filepath.Join(homeDir, "/.strike-keys/")
	fullPath := filepath.Join(strikeKeyDir, keyNameDotPem)

	// Check if key directory exists
	if _, err := os.Stat(strikeKeyDir); os.IsNotExist(err) {
		// Directory doesn't exist
		fmt.Println("~/.strike_keys not found. Creating ~/.strike_keys")
		//TODO:Hidden Home for now, handle storing this in Library/Application Support : Cross Platform
		err = os.Mkdir(strikeKeyDir, 0755)
		if err != nil {
			return fmt.Errorf("error creating key directory: %v", err)
		}
	} else if err == nil {
		fmt.Println("~/.strike_keys found, Writing keys")
	}

	keyPEM := pem.Block{
		Type:  keyType,
		Bytes: keyBytes,
	}

	err = os.WriteFile(fullPath, pem.EncodeToMemory(&keyPEM), 0600)
	if err != nil {
		return fmt.Errorf("failed to write public key: %v", err)
	}

	fmt.Printf("Strike Key \"%v\" generated and saved to: %v\n", keyNameDotPem, strikeKeyDir)
	return nil
}
