package main

import (
	"bufio"
	"context"
	"database/sql"
	"embed"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/JohnnyGlynn/strike/internal/client"
	"github.com/JohnnyGlynn/strike/internal/client/types"
	"github.com/JohnnyGlynn/strike/internal/config"
	"github.com/JohnnyGlynn/strike/internal/keys"
	"github.com/google/uuid"

	pb "github.com/JohnnyGlynn/strike/msgdef/message"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	_ "modernc.org/sqlite"
)

//go:embed client.sql
var schemaFS embed.FS
var cDB *sql.DB

func main() {
	fmt.Println("Strike client")

	// Avoid shadowing
	var clientCfg config.ClientConfig
	var err error

	idb, err := initDB("./client.db")
	if err != nil {
		log.Fatalf("Error initializing db: %v", err)
	}

	defer func() {
		if dbInitErr := idb.Close(); dbInitErr != nil {
			log.Fatalf("error initializing db: %v\n", dbInitErr)
		}
	}()

	cDB = idb

	configFilePath := flag.String("config", "", "Path to configuration JSON file")
	keygen := flag.Bool("keygen", false, "Launch Strike Key generation, creating keypair for user not bringing existing PKI")
	flag.Parse()

	/*
		Flag check:
		-config: Provide a config file, otherwise look for env vars
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

	statements, err := client.PrepareStatements(context.TODO(), cDB)
	if err != nil {
		log.Fatalf("Failed to prepare statements: %v", err)
	}

	defer func() {
		if psErr := client.CloseStatements(statements); psErr != nil {
			log.Fatalf("error preparing statements: %v\n", psErr)
		}
	}()

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

	defer func() {
		if connectionError := conn.Close(); connectionError != nil {
			log.Fatalf("Failed to connect to Strike Server: %v\n", connectionError)
		}
	}()

	newClient := pb.NewStrikeClient(conn)

	clientInfo := &types.ClientInfo{
		Config: &clientCfg,
		Keys:   loadedKeys,
		Cache: types.Cache{
			Chats:          make(map[uuid.UUID]*pb.Chat),
			FriendRequests: make(map[uuid.UUID]*pb.FriendRequest),
		},
		Username:    "",
		UserID:      uuid.Nil,
		Pbclient:    newClient,
		Pstatements: statements,
		Shell:       &types.ShellState{},
	}

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

				err = client.Login(clientInfo, password)
				if err != nil {
					log.Fatalf("error during login: %v", err)
				}
				isLoggedIn = true

				go func() {
					err = client.RegisterStatus(clientInfo)
					if err != nil {
						log.Fatalf("error connecting stream: %v", err)
					}
				}()

				fmt.Println("Logged In!")
				// Logged in
				fmt.Printf("Welcome back %s!\n", clientInfo.Username)
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

				// Create a new UUID
				clientInfo.UserID = uuid.New()
				clientInfo.Username = username

				err = client.ClientSignup(clientInfo, password, loadedKeys["EncryptionPublicKey"], loadedKeys["SigningPublicKey"])
				if err != nil {
					log.Fatalf("error connecting: %v", err)
				}

				isLoggedIn = true

				go func() {
					err = client.RegisterStatus(clientInfo)
					if err != nil {
						log.Fatalf("error connecting stream: %v", err)
					}
				}()

				fmt.Println("Logged In!")
				// Logged in
				fmt.Printf("Welcome %s!\n", username)
			case "/exit":
				fmt.Println("Strike Client shutting down")
				return
			default:
				fmt.Printf("Unknown command: %s\n", input)
			}

		} else {
			client.MShell(clientInfo)
		}
	}
}

func initDB(path string) (*sql.DB, error) {
	firstRun := false

	if _, err := os.Stat(path); os.IsNotExist(err) {
		firstRun = true
	}

	dbOpen, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open db")
	}

	if firstRun {
		init, err := schemaFS.ReadFile("client.sql")
		if err != nil {
			return nil, fmt.Errorf("schema not found")
		}

		_, err = dbOpen.Exec(string(init))
		if err != nil {
			return nil, fmt.Errorf("failed to init db")
		}
	}

	return dbOpen, nil
}
