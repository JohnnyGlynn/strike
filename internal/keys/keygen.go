package keys

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
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
