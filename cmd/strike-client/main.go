package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/JohnnyGlynn/strike/internal/client"
	"github.com/JohnnyGlynn/strike/internal/config"
	"github.com/JohnnyGlynn/strike/internal/db"
	"github.com/JohnnyGlynn/strike/internal/keys"
	"github.com/JohnnyGlynn/strike/internal/types"
	"github.com/google/uuid"

	pb "github.com/JohnnyGlynn/strike/msgdef/message"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
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

	// TODO: Seperate client db
	pgConfig, err := pgxpool.ParseConfig("postgres://strikeadmin:plaintextisbad@localhost:5432/strike")
	if err != nil {
		log.Fatalf("Config parsing failed: %v", err)
	}

	pool, err := pgxpool.NewWithConfig(context.TODO(), pgConfig)
	if err != nil {
		log.Fatalf("DB pool connection failed: %v", err)
	}

	defer pool.Close()

	statements, err := db.PreparedClientStatements(context.TODO(), pool)
	if err != nil {
		log.Fatalf("Failed to prepare client statements: %v", err)
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

	defer func() {
		if connectionError := conn.Close(); connectionError != nil {
			log.Fatalf("Failed to connect to Strike Server: %v\n", connectionError)
		}
	}()

	newClient := pb.NewStrikeClient(conn)

	clientInfo := &types.ClientInfo{
		Config: &clientCfg,
		Keys:   loadedKeys,
		Cache: types.ClientCache{
			Invites: make(map[uuid.UUID]*pb.BeginChatRequest),
			Chats:   make(map[uuid.UUID]*pb.Chat),
		},
		Username:    "",
		UserID:      uuid.Nil,
		Pbclient:    newClient,
		Pstatements: statements,
		DBpool:      pool,
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

				// Retrieve UUID
				var userID uuid.UUID
				err = clientInfo.DBpool.QueryRow(context.TODO(), clientInfo.Pstatements.GetUserId, clientInfo.Username).Scan(&userID)
				if err != nil {
					if pgerr, ok := err.(*pgconn.PgError); ok && pgerr.Code == "no-data-found" {
						log.Fatalf("DB error: %v", err)
					}
					log.Fatalf("An Error occured while logging in: %v", err)
				}

				clientInfo.UserID = userID

				// Spawn a goroutine so we can have the login function maintain userstatus stream aka Online
				// TODO: Clean this up, Login isnt really correct now, RegisterStatus?
				go func() {
					// Login now connects to UserStatus stream to show wheter user is online
					err = client.Login(clientInfo, password)
					if err != nil {
						log.Fatalf("error connecting: %v", err)
					}
				}()

				isLoggedIn = true
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
				newUserID := uuid.New()
				clientInfo.UserID = newUserID
				clientInfo.Username = username

				err = client.ClientSignup(clientInfo, password, loadedKeys["EncryptionPublicKey"], loadedKeys["SigningPublicKey"])
				if err != nil {
					log.Fatalf("error connecting: %v", err)
				}

				go func() {
					// Login now connects to UserStatus stream to show wheter user is online
					err = client.Login(clientInfo, password)
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
			client.MessagingShell(clientInfo)
		}
	}
}
