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

	// Avoid shadowing
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

	// If user wants to create keys to use with strike - no existing PKI
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

	keysMap := map[string]keys.KeyDefinition{
		"SigningPrivateKey":    {Path: clientCfg.SigningPrivateKeyPath, Type: keys.SigningKey},
		"SigningPublicKey":     {Path: clientCfg.SigningPublicKeyPath, Type: keys.SigningKey},
		"EncryptionPrivateKey": {Path: clientCfg.EncryptionPrivateKeyPath, Type: keys.EncryptionKey},
		"EncryptionPublicKey":  {Path: clientCfg.EncryptionPublicKeyPath, Type: keys.EncryptionKey},
	}

	loadedKeys, err := keys.LoadAndValidateKeys(keysMap)
	if err != nil {
		log.Fatalf("error loading and validating keys: %v", err)
	}

	clientInfo := &client.ClientInfo{
		Config: &clientCfg,
		Keys:   loadedKeys,
		Cache: client.ClientCache{
			Invites: make(map[string]*pb.BeginChatRequest),
			Chats:   make(map[string]*pb.Chat),
		},
		Username: "",
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

	isLoggedIn := false

	var username string

	fmt.Println("Type /login to log into the Strike Messaging service")
	fmt.Println("Type /signup to signup to the Strike Messaging service")
	fmt.Println("Type /exit to quit.")

	for {
		if !isLoggedIn {
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

			switch input {
			case "/login":
				inputReader.Reset(os.Stdin)

				username, err = client.LoginInput("Username > ", inputReader)
				if err != nil {
					log.Printf("error reading username: %v\n", err)
					continue
				}

				password, err := client.LoginInput("Password > ", inputReader)
				if err != nil {
					log.Printf("error reading username: %v\n", err)
					continue
				}

				if username == "" || password == "" {
					fmt.Println("Username and password cannot be empty.")
					continue
				}

				clientInfo.Username = username

				// Spawn a goroutine so we can have the login function maintain userstatus stream aka Online
				// TODO: Clean this up, Login isnt really correct now, RegisterStatus?
				go func() {
					// Login now connects to UserStatus stream to show wheter user is online
					err = client.Login(newClient, username, password)
					if err != nil {
						log.Fatalf("error connecting: %v", err)
					}
				}()
				isLoggedIn = true
				fmt.Println("Logged In!")
				// Logged in
				fmt.Printf("Welcome back %s!\n", username)
			case "/signup":
				inputReader.Reset(os.Stdin)

				username, err = client.LoginInput("Username > ", inputReader)
				if err != nil {
					log.Printf("error reading username: %v\n", err)
					continue
				}

				password, err := client.LoginInput("Password > ", inputReader)
				if err != nil {
					log.Printf("error reading username: %v\n", err)
					continue
				}

				if username == "" || password == "" {
					fmt.Println("Username and password cannot be empty.")
					continue
				}

				err = client.ClientSignup(newClient, username, password, loadedKeys["EncryptionPublicKey"], loadedKeys["SigningPublicKey"])
				if err != nil {
					log.Fatalf("error connecting: %v", err)
				}

				clientInfo.Username = username

				go func() {
					// Login now connects to UserStatus stream to show wheter user is online
					err = client.Login(newClient, username, password)
					if err != nil {
						log.Fatalf("error connecting: %v", err)
					}
				}()
				isLoggedIn = true
				fmt.Println("Logged In!")
				// Logged in
				fmt.Printf("Welcome back %s!\n", username)
			case "/exit":
				fmt.Println("Strike Client shutting down")
				return
			default:
				fmt.Printf("Unknown command: %s\n", input)
			}

		} else {

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

			switch input {
			case "/msgshell":
				client.MessagingShell(*clientInfo)
			case "/exit":
				fmt.Println("Strike Client shutting down")
				return
			default:
				fmt.Printf("Unknown command: %s\n", input)
			}

		}
	}
}
