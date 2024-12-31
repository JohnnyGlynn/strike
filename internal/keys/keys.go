package keys

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"os"
)

func Keygen() error {

	//TODO: There is definetly a better way to do this
	fmt.Println("WARNING: You (the user) are responsible for the safety of these key files. You will not be able to recover these files if they are lost")

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("error Generating key: %v", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("error finding user home directory: %v", err)
	}

	//TODO:Hidden Home for now, handle storing this in Library/Application Support : Cross Platform
	err = os.Mkdir(homeDir+"/.strike-keys", 0755)
	if err != nil {
		return fmt.Errorf("error creating key directory: %v", err)
	}

	//Private
	privateKeyPEM := pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privateKey.Seed(),
	}
	err = os.WriteFile(homeDir+"/.strike-keys/strike.pem", pem.EncodeToMemory(&privateKeyPEM), 0600)
	if err != nil {
		return fmt.Errorf("failed to write private key: %v", err)
	}

	//Public
	publicKeyPEM := pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKey,
	}
	err = os.WriteFile(homeDir+"/.strike-keys/strike_public.pem", pem.EncodeToMemory(&publicKeyPEM), 0600)
	if err != nil {
		return fmt.Errorf("failed to write public key: %v", err)
	}

	fmt.Printf("Strike Keys generated and saved to: %v/strike-keys\n", homeDir)
	return nil

}

func GetKeyFromPath(path string) ([]byte, error) {
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

func ValidateKeys(keyBytes []byte) error {
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
