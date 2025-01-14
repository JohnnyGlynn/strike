package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

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

	inputReader := bufio.NewReader(os.Stdin)

	fmt.Println("Type /login to log into the Strike Messaging service")
	fmt.Println("Type /exit to quit.")

	for {
		// Prompt for input
		fmt.Print("> ")
		input, err := inputReader.ReadString('\n')
		if err != nil {
			log.Printf("Error reading input: %v\n", err)
			continue
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		commandAndArgs := strings.SplitN(input, " ", 2) //Check for space then splint into command and argument
		command := commandAndArgs[0]
		var arg string //Make it exist
		if len(commandAndArgs) > 1 {
			arg = commandAndArgs[1]
		}

		switch command {
		case "/login":
			//Spawn a goroutine so we can have the login function maintain userstatus stream aka Online
			//TODO: Clean this up, Login isnt really correct now, RegisterStatus?
			go func() {
				//Login now connects to UserStatus stream to show wheter user is online
				err = client.Login(newClient, clientCfg.Username)
				if err != nil {
					log.Fatalf("error connecting: %v", err)
				}
			}()
		case "/beginchat":
			if arg == "" {
				fmt.Println("Usage: /beginchat <username you want to chat with>")
				continue
			}
			go func() {
				err = client.BeginChat(newClient, clientCfg.Username, arg)
				//TODO: Not fatal?
				if err != nil {
					log.Fatalf("error beginning chat: %v", err)
				}
			}()
		case "/exit":
			fmt.Println("Strike Client shutting down")
			return
		default:
			fmt.Printf("Unknown command: %s\n", input)
		}
	}

	//TODO: Gate Signup with Login - i.e. Try to login, if user not found, signup, then login
	// err = client.ClientSignup(newClient, clientCfg.Username, encryptionPublicKey, signingPublicKey)
	// if err != nil {
	// 	log.Fatalf("error with client signup: %v", err)
	// }

	// go func() {
	// 	// Disable auto chat for now
	// 	err = client.AutoChat(newClient, clientCfg.Username, signingPublicKey)
	// 	if err != nil {
	// 		log.Fatalf("error starting AutoChat: %v", err)
	// 	}
	// }()

	// time.Sleep(30 * time.Second)
	// fmt.Println("ATTEMPTING TO BEGIN CHAT")

	// go func() {
	// 	client.ConnectMessageStream(newClient, clientCfg.Username)
	// }()

	// //TODO: make configurable, allow  for client input
	// err = client.BeginChat(newClient, clientCfg.Username, "client0")
	// if err != nil {
	// 	log.Fatalf("error begining chat: %v", err)
	// }

	// fmt.Println("ATTEMPTING TO CONFIRM CHAT")
	// time.Sleep(30 * time.Second)

}
