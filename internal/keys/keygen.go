package keys

import (
	"crypto/ed25519"
	"fmt"
	"os"
)

func Keygen() string {

	var keyFileName string

	// TODO: Ask user if they want to generate, or add their own public key
	public, private, err := ed25519.GenerateKey(nil)
	if err != nil {
		fmt.Printf("Error Generating key: %v", err)
	}

	fmt.Println("WARNING: You (the user) are responsible for the safety of these key files")
	//user input for filename
	fmt.Println("Please enter a name for your keyfiles:")
	fmt.Scan(&keyFileName)

	//public key
	publicKeyFile, err := os.Create("./cfg/" + keyFileName + ".pub")
	if err != nil {
		fmt.Printf("Error creating public key file: %v", err)
	}
	defer publicKeyFile.Close()

	//private key
	privateKeyFile, err := os.Create("./cfg/" + keyFileName)
	if err != nil {
		fmt.Printf("Error creating private key file: %v", err)
	}
	defer privateKeyFile.Close()

	//Writing the keys to files as byte arrays
	//Bad Idea? Can't be used elsewhere?
	//TODO: Fix this?
	_, err = publicKeyFile.Write(public)
	if err != nil {
		fmt.Printf("Error writing private key: %v", err)
	}

	_, err = privateKeyFile.Write(private)
	if err != nil {
		fmt.Printf("Error writing private key: %v", err)
	}

	//TODO: write the path to config
	return "./cfg/" + keyFileName + ".pub"

}
