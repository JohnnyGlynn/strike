package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/JohnnyGlynn/strike/internal/client"
	"github.com/JohnnyGlynn/strike/internal/keys"

	pb "github.com/JohnnyGlynn/strike/msgdef/message"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	//Avoid shadowing
	var config *client.Config
	var err error

	fmt.Println("Strike client")

	configFilePath := flag.String("config", "", "Path to configuration JSON file")
	keygen := flag.Bool("keygen", false, "Launch Strike Key generation, creating keypair for user not bringing existing PKI")
	flag.Parse()

	/*
		Flag check - Provide a config file, otherwise look for env vars
		Average user who just wants to connect to a server can run binary+json file,
		meanwhile running a server you can have a client contianer present with env vars provided to pod
	*/

	if *configFilePath != "" {
		log.Println("Loading Config from File")
		config, err = client.LoadConfigFile(*configFilePath)
		if err != nil {
			log.Fatalf("Failed to load client config: %v", err)
		}
		//TODO: Looks gross
		if err := config.ValidateConfig(); err != nil {
			log.Fatalf("Invalid client config: %v", err)
		}
	} else {
		log.Println("Loading Config from Envrionment Variables")
		config = client.LoadConfigEnv()
		if err := config.ValidateConfig(); err != nil {
			log.Fatalf("Invalid client config: %v", err)
		}
	}

	//If user wants to create keys to use with strike - no existing PKI
	if *keygen {
		err := keys.Keygen()
		if err != nil {
			fmt.Printf("error generating keys: %v\n", err)
			return
		}
		fmt.Println("User keys generated successfully ")
		os.Exit(0)
	}

	// +v to print struct fields too
	log.Printf("Loaded client Config: %+v", config)

	//Get our keys then validate them - facilitates BYO pki compliance to ed25519
	publicKey, err := keys.GetKeyFromPath(config.PublicKeyPath)
	if err != nil {
		log.Fatalf("failed to get public key: %v", err)
	}

	err = keys.ValidateKeys(publicKey)
	if err != nil {
		log.Fatalf("failed to validate public key: %v", err)
	}

	privateKey, err := keys.GetKeyFromPath(config.PrivateKeyPath)
	if err != nil {
		log.Fatalf("failed to get public key: %v", err)
	}

	err = keys.ValidateKeys(privateKey)
	if err != nil {
		log.Fatalf("failed to validate private key: %v", err)
	}

	// Begin GRPC setup
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))

	conn, err := grpc.NewClient(config.ServerHost, opts...)
	if err != nil {
		log.Fatalf("fail to dial: %v", err)
	}
	defer conn.Close()

	newClient := pb.NewStrikeClient(conn)

	// client.RegisterClient(newClient, config.Username, pubkey)
	err = client.Login(newClient, config.Username, publicKey)
	if err != nil {
		log.Fatalf("error logging in: %v", err)
	}

	err = client.AutoChat(newClient, config.Username, publicKey)
	if err != nil {
		log.Fatalf("error starting AutoChat: %v", err)
	}
}
