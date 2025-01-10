package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/JohnnyGlynn/strike/internal/client"
	"github.com/JohnnyGlynn/strike/internal/config"
	"github.com/JohnnyGlynn/strike/internal/keys"

	pb "github.com/JohnnyGlynn/strike/msgdef/message"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func main() {
	fmt.Println("Strike client")

	//Avoid shadowing
	var clientCfg config.ClientConfig
	var err error

	configFilePath := flag.String("config", "", "Path to configuration JSON file")
	keygen := flag.Bool("keygen", false, "Launch Strike Key generation, creating keypair for user not bringing existing PKI")
	flag.Parse()

	/*
		Flag check:
		-config: Provide a config file, otherwise look for env vars
		Average user who just wants to connect to a server can run binary+json file,
		meanwhile running a server you can have a client contianer present with env vars provided to pod

		-keygen: Generate user keys
	*/

	//If user wants to create keys to use with strike - no existing PKI
	if *keygen {
		err := keys.SigningKeygen()
		if err != nil {
			fmt.Printf("error generating signing keys: %v\n", err)
			return
		}

		fmt.Println("Signing keys generated successfully ")

		err = keys.EncryptionKeygen()
		if err != nil {
			fmt.Printf("error generating encryption keys: %v\n", err)
			return
		}

		fmt.Println("Encryption keys generated successfully ")
		os.Exit(0)
	}

	if *configFilePath != "" && !*keygen {
		log.Println("Loading Config from File")

		clientCfg, err = config.LoadConfigFile[config.ClientConfig](*configFilePath)
		if err != nil {
			log.Fatalf("Failed to load client config: %v", err)
		}

		if err := clientCfg.ValidateConfig(); err != nil {
			log.Fatalf("Invalid client config: %v", err)
		}
	} else if !*keygen {
		log.Println("Loading Config from Envrionment Variables")

		clientCfg = *config.LoadClientConfigEnv()

		if err := clientCfg.ValidateEnv(); err != nil {
			log.Fatalf("Invalid client config: %v", err)
		}
	}

	// +v to print struct fields too
	log.Printf("Loaded client Config: %+v", clientCfg)

	//TODO: Clean up these calls to read and validate keys

	//Get our keys then validate them - facilitates BYO pki compliance to ed25519 and curve25519
	signingPublicKey, err := keys.GetKeyFromPath(clientCfg.SigningPublicKeyPath)
	if err != nil {
		log.Fatalf("failed to get public signing key: %v", err)
	}

	err = keys.ValidateSigningKeys(signingPublicKey)
	if err != nil {
		log.Fatalf("failed to validate public signing key: %v", err)
	}

	signingPrivateKey, err := keys.GetKeyFromPath(clientCfg.SigningPrivateKeyPath)
	if err != nil {
		log.Fatalf("failed to get private signing key: %v", err)
	}

	err = keys.ValidateSigningKeys(signingPrivateKey)
	if err != nil {
		log.Fatalf("failed to validate private signing key: %v", err)
	}

	encryptionPrivateKey, err := keys.GetKeyFromPath(clientCfg.EncryptionPrivateKeyPath)
	if err != nil {
		log.Fatalf("failed to get private encryption key: %v", err)
	}

	err = keys.ValidateEncryptionKeys(encryptionPrivateKey)
	if err != nil {
		log.Fatalf("failed to validate private encryption key: %v", err)
	}

	encryptionPublicKey, err := keys.GetKeyFromPath(clientCfg.EncryptionPublicKeyPath)
	if err != nil {
		log.Fatalf("failed to get public encryption key: %v", err)
	}

	err = keys.ValidateEncryptionKeys(encryptionPublicKey)
	if err != nil {
		log.Fatalf("failed to validate public encryption key: %v", err)
	}

	// Begin GRPC setup
	creds, err := credentials.NewClientTLSFromFile(clientCfg.ServerCertificatePath, "")
	if err != nil {
		log.Fatalf("Failed to load server certificate: %v", err)
	}

	var opts []grpc.DialOption
	opts = append(opts, grpc.WithTransportCredentials(creds))

	conn, err := grpc.NewClient(clientCfg.ServerHost, opts...)
	if err != nil {
		log.Fatalf("fail to dial: %v", err)
	}
	defer conn.Close()

	newClient := pb.NewStrikeClient(conn)

	//TODO: Gate Signup with Login - i.e. Try to login, if user not found, signup, then login
	err = client.ClientSignup(newClient, clientCfg.Username, encryptionPublicKey, signingPublicKey)
	if err != nil {
		log.Fatalf("error with client signup: %v", err)
	}

	//Login now connects to UserStatus stream to show wheter user is online
	err = client.Login(newClient, clientCfg.Username)
	if err != nil {
		log.Fatalf("error connecting: %v", err)
	}

	//Disable auto chat for now
	// err = client.AutoChat(newClient, clientCfg.Username, signingPublicKey)
	// if err != nil {
	// 	log.Fatalf("error starting AutoChat: %v", err)
	// }
}
