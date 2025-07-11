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

// var cDB *sql.DB

func main() {
	fmt.Println("Strike client")

	idb, err := initDB("./client.db")
	if err != nil {
		fmt.Printf("Error initializing db: %v\n", err)
		return
	}

	defer func() {
		if dbInitErr := idb.Close(); dbInitErr != nil {
			fmt.Printf("error initializing db: %v\n", dbInitErr)
			return
		}
	}()

	// cDB = idb

	configFilePath := flag.String("config", "", "Path to configuration JSON file")
	keygen := flag.Bool("keygen", false, "Launch Strike Key generation, creating keypair for user not bringing existing PKI")
	flag.Parse()

	clientCfg, loadedKeys, err := setupClientConfigAndKeys(*configFilePath, *keygen)
	if err != nil {
		fmt.Printf("error setting up config/keys: %v", err)
		return
	}

	statements, err := client.PrepareStatements(context.TODO(), idb)
	if err != nil {
		fmt.Printf("Failed to prepare statements: %v\n", err)
		return
	}

	defer func() {
		if psErr := client.CloseStatements(statements); psErr != nil {
			fmt.Printf("error preparing statements: %v\n", psErr)
			return
		}
	}()

	// Begin GRPC setup
	creds, err := credentials.NewClientTLSFromFile(clientCfg.ServerCertificatePath, "")
	if err != nil {
		fmt.Printf("Failed to load server certificate: %v\n", err)
		return
	}

	var opts []grpc.DialOption
	opts = append(opts, grpc.WithTransportCredentials(creds))

	conn, err := grpc.NewClient(clientCfg.ServerHost, opts...)
	if err != nil {
		fmt.Printf("fail to dial: %v\n", err)
		return
	}

	defer func() {
		if connectionError := conn.Close(); connectionError != nil {
			fmt.Printf("Failed to connect to Strike Server: %v\n", connectionError)
			return
		}
	}()

	newClient := pb.NewStrikeClient(conn)

	clientInfo := &types.ClientInfo{
		Config: &clientCfg,
		Keys:   loadedKeys,
		Cache: types.Cache{
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

			case "/signup":
				inputReader.Reset(os.Stdin)

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
			return nil, err
		}
	}

	return dbOpen, nil
}

func setupClientConfigAndKeys(cfgPath string, keygen bool) (config.ClientConfig, map[string][]byte, error) {
	// Avoid shadowing
	var clientCfg config.ClientConfig
	var err error

	// TODO: Replace Keygen with --firstrun?
	if keygen {
		if err := keys.SigningKeygen(); err != nil {
			return clientCfg, nil, fmt.Errorf("error generating signing keys: %v\n", err)
		}
		fmt.Println("Signing keys generated successfully ")

		if err = keys.EncryptionKeygen(); err != nil {
			return clientCfg, nil, fmt.Errorf("error generating encryption keys: %v\n", err)
		}
		fmt.Println("Encryption keys generated successfully ")

		os.Exit(0)
	}

	if cfgPath != "" {
		fmt.Println("Loading Config from File")

		clientCfg, err = config.LoadConfigFile[config.ClientConfig](cfgPath)
		if err != nil {
			return clientCfg, nil, fmt.Errorf("Failed to load client config: %v\n", err)
		}

		if err := clientCfg.ValidateConfig(); err != nil {
			return clientCfg, nil, fmt.Errorf("Invalid client config: %v\n", err)
		}

	} else {
		log.Println("Loading Config from Envrionment Variables")

		clientCfg = *config.LoadClientConfigEnv()

		if err := clientCfg.ValidateEnv(); err != nil {
			return clientCfg, nil, fmt.Errorf("Invalid client config: %v\n", err)
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
		return clientCfg, nil, fmt.Errorf("error loading and validating keys: %v\n", err)
	}

	return clientCfg, loadedKeys, nil

}

func handleLogin(reader *bufio.Reader, clientInfo *types.ClientInfo) error {
	username, err := client.LoginInput("Username > ", reader)
	if err != nil {
		return fmt.Errorf("error reading username: %v", err)

	}

	password, err := client.LoginInput("Password > ", reader)
	if err != nil {
		return fmt.Errorf("error reading username: %v", err)
	}

	if username == "" || password == "" {
		return fmt.Errorf("Username and password cannot be empty.")
	}

	clientInfo.Username = username

	err = client.Login(clientInfo, password)
	if err != nil {
		return fmt.Errorf("error during login: %v", err)
	}

	go func() {
		if err = client.RegisterStatus(clientInfo); err != nil {
			fmt.Printf("error connecting stream: %v", err)
			return
		}
	}()

	fmt.Printf("Welcome back %s!\n", clientInfo.Username)

	return nil
}

func handleSignup(reader *bufio.Reader, clientInfo *types.ClientInfo) error {
	username, err := client.LoginInput("Username > ", reader)
	if err != nil {
		return fmt.Errorf("error reading username: %v\n", err)
	}

	password, err := client.LoginInput("Password > ", reader)
	if err != nil {
		return fmt.Errorf("error reading username: %v\n", err)
	}

	if username == "" || password == "" {
		return fmt.Errorf("Username and password cannot be empty.")
	}

	// Create a new UUID
	clientInfo.UserID = uuid.New()
	clientInfo.Username = username
	//TODO: Handle Keyloading here?

	//WARN: Keys are empty here
	err = client.ClientSignup(clientInfo, password, clientInfo.Keys["EncryptionPublicKey"], clientInfo.Keys["SigningPublicKey"])
	if err != nil {
		return fmt.Errorf("error connecting: %v\n", err)
	}

	go func() {
		if err = client.RegisterStatus(clientInfo); err != nil {
			fmt.Printf("error connecting stream: %v\n", err)
			return
		}
	}()

	fmt.Printf("Welcome %s!\n", username)

	return nil
}
