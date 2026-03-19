package keys

import (
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

type KeyType int

// enum
const (
	SigningKey    KeyType = iota
	EncryptionKey KeyType = iota
)

type KeyDefinition struct {
	Path string
	Type KeyType
}

func SigningKeygen(outputDir string) error {
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

	err = writeToPem(privateKeyBytes, "PRIVATE KEY", "strike_signing.pem", outputDir)
	if err != nil {
		return fmt.Errorf("failed to write private key: %v", err)
	}

	// Encode PKIX format
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return fmt.Errorf("error encoding public key: %v", err)
	}

	err = writeToPem(publicKeyBytes, "PUBLIC KEY", "strike_public_signing.pem", outputDir)
	if err != nil {
		return fmt.Errorf("failed to write public key: %v", err)
	}

	fmt.Printf("Strike Signing Keys generated and saved to %s\n", outputDir)
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
		// Check key type
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return fmt.Errorf("failed to parse private key: %w", err)
		}
		// ok if ed25519
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

func EncryptionKeygen(outputDir string) error {
	curve := ecdh.X25519()

	privateKey, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("error Generating encryption key: %v", err)
	}

	// Extract the private and public keys as byte slices
	privateKeyBytes := privateKey.Bytes()
	err = writeToPem(privateKeyBytes, "PRIVATE KEY", "strike_encryption.pem", outputDir)
	if err != nil {
		return fmt.Errorf("failed to write private key: %v", err)
	}

	publicKeyBytes := privateKey.PublicKey().Bytes()
	err = writeToPem(publicKeyBytes, "PUBLIC KEY", "strike_public_encryption.pem", outputDir)
	if err != nil {
		return fmt.Errorf("failed to write private key: %v", err)
	}

	fmt.Printf("Strike Encryption Keys generated and saved to %s\n", outputDir)
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

func LoadAndValidateKeys(keyMap map[string]KeyDefinition) (map[string][]byte, error) {
	loadedKeys := make(map[string][]byte)

	for name, def := range keyMap {
		key, err := GetKeyFromPath(def.Path)
		if err != nil {
			fmt.Printf("failed to read key from path: %v", err)
			return nil, err
		}

		switch def.Type {
		case SigningKey:
			err = ValidateSigningKeys(key)
		case EncryptionKey:
			err = ValidateEncryptionKeys(key)
		default:
			return nil, fmt.Errorf("unknown type for key %s", name)
		}

		if err != nil {
			return nil, fmt.Errorf("failed to validate %s: %v", name, err)
		}

		loadedKeys[name] = key
	}

	return loadedKeys, nil
}

func GetKeyFromPath(path string) ([]byte, error) {
	keyFile, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("error opening key file: %v", err)
	}

	defer func() {
		if fileError := keyFile.Close(); fileError != nil {
			fmt.Printf("error reading file: %v\n", fileError)
		}
	}()

	key, err := io.ReadAll(keyFile)
	if err != nil {
		return nil, fmt.Errorf("error reading key file: %v", err)
	}

	return key, nil
}

func writeToPem(keyBytes []byte, keyType string, keyNameDotPem string, outputDir string) error {
	fullPath := filepath.Join(outputDir, keyNameDotPem)

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("error creating key directory: %v", err)
	}

	keyPEM := pem.Block{
		Type:  keyType,
		Bytes: keyBytes,
	}

	err := os.WriteFile(fullPath, pem.EncodeToMemory(&keyPEM), 0600)
	if err != nil {
		return fmt.Errorf("failed to write key: %v", err)
	}

	fmt.Printf("Strike Key \"%v\" saved to: %v\n", keyNameDotPem, outputDir)
	return nil
}

func GenerateServerKeysAndCert(outputDir string) error {
	fmt.Println("Server Keys and Cert Generator")

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("error Generating Server Signing keys: %v", err)
	}

	// Encode PKCS#8 format
	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return fmt.Errorf("error encoding private key: %v", err)
	}

	// Encode PKIX format
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return fmt.Errorf("error encoding public key: %v", err)
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("error creating key directory: %v", err)
	}

	pubFullPath := filepath.Join(outputDir, "strike_server_public.pem")
	privFullPath := filepath.Join(outputDir, "strike_server.pem")
	certFullPath := filepath.Join(outputDir, "strike_server.crt")

	pubKeyPEM := pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	}

	privKeyPEM := pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privateKeyBytes,
	}

	err = os.WriteFile(pubFullPath, pem.EncodeToMemory(&pubKeyPEM), 0600)
	if err != nil {
		return fmt.Errorf("failed to write server public key: %v", err)
	}

	err = os.WriteFile(privFullPath, pem.EncodeToMemory(&privKeyPEM), 0600)
	if err != nil {
		return fmt.Errorf("failed to write server private key: %v", err)
	}

	// Generate x509 serial no bigger than 20 bytes
	twentyBytes := new(big.Int).Lsh(big.NewInt(1), 160)
	serialNumber, err := rand.Int(rand.Reader, twentyBytes)
	if err != nil {
		return err
	}

	// Make sure non-negative
	if serialNumber.Sign() < 0 {
		serialNumber.Abs(serialNumber)
	}

	// Server template - TODO: Make the CommonName/DNSNames configurable during keygen
	strikeCert := x509.Certificate{
		SerialNumber: serialNumber,
		Subject:      pkix.Name{CommonName: "strike-server"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour), // 1 year
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageClientAuth,
			x509.ExtKeyUsageServerAuth,
		},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost"},
	}

	// Self-sign TODO: More robust (i.e no self parent)
	signedCert, err := x509.CreateCertificate(rand.Reader, &strikeCert, &strikeCert, publicKey, privateKey)
	if err != nil {
		fmt.Printf("Failed to create server certificate: %v", err)
		return nil
	}

	certPEM := pem.Block{
		Type:  "CERTIFICATE",
		Bytes: signedCert,
	}

	err = os.WriteFile(certFullPath, pem.EncodeToMemory(&certPEM), 0600)
	if err != nil {
		return fmt.Errorf("failed to create server.crt: %v", err)
	}

	fmt.Printf("Strike Server Signing Keys and Certificate generated and saved to %s\n", outputDir)
	return nil
}
