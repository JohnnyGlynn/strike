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

	schema, err := schemaFS.ReadFile("client.sql")
	if err != nil {
		fmt.Println("schema not found")
		return
	}

	idb, err := initDB("./client.db", schema)
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
		fmt.Printf("error setting up config/keys: %v\n", err)
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

	conn, err := grpcSetup(clientCfg)
	if err != nil {
		fmt.Printf("error establishing grpc conncetion: %v\n", err)
		return
	}

	defer func() {
		if connectionError := conn.Close(); connectionError != nil {
			fmt.Printf("Failed to connect to Strike Server: %v\n", connectionError)
			return
		}
	}()

	client := pb.NewStrikeClient(conn)

	clientInfo := &types.ClientInfo{
		Config: &clientCfg,
		Keys:   loadedKeys,
		Cache: types.Cache{
			FriendRequests: make(map[uuid.UUID]*pb.FriendRequest),
		},
		Username:    "",
		UserID:      uuid.Nil,
		Pbclient:    client,
		Pstatements: statements,
		Shell:       &types.ShellState{},
	}

	if err := launchREPL(clientInfo); err != nil {
		fmt.Printf("repl error: %v\n", err)
		return
	}
}

func initDB(path string, schema []byte) (*sql.DB, error) {
	firstRun := false

	if _, err := os.Stat(path); os.IsNotExist(err) {
		firstRun = true
	}

	dbOpen, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open db")
	}

	if firstRun {
		_, err = dbOpen.Exec(string(schema))
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
			return clientCfg, nil, fmt.Errorf("error generating signing keys: %v", err)
		}
		fmt.Println("Signing keys generated successfully ")

		if err = keys.EncryptionKeygen(); err != nil {
			return clientCfg, nil, fmt.Errorf("error generating encryption keys: %v", err)
		}
		fmt.Println("Encryption keys generated successfully")

		os.Exit(0)
	}

	if cfgPath != "" {
		clientCfg, err = config.LoadConfigFile[config.ClientConfig](cfgPath)
		if err != nil {
			return clientCfg, nil, fmt.Errorf("failed to load client config: %v", err)
		}

		if err := clientCfg.ValidateConfig(); err != nil {
			return clientCfg, nil, fmt.Errorf("invalid client config: %v", err)
		}

	} else {
		clientCfg = *config.LoadClientConfigEnv()

		if err := clientCfg.ValidateEnv(); err != nil {
			return clientCfg, nil, fmt.Errorf("invalid client config: %v", err)
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
		return clientCfg, nil, fmt.Errorf("error loading and validating keys: %v", err)
	}

	return clientCfg, loadedKeys, nil

}

func grpcSetup(cfg config.ClientConfig) (*grpc.ClientConn, error) {

	creds, err := credentials.NewClientTLSFromFile(cfg.ServerCertificatePath, "")
	if err != nil {
		return nil, fmt.Errorf("failed to load server certificate: %v", err)
	}

	var opts []grpc.DialOption
	opts = append(opts, grpc.WithTransportCredentials(creds))

	conn, err := grpc.NewClient(cfg.ServerHost, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to dial: %v", err)
	}

	return conn, nil
}

func handleLogin(reader *bufio.Reader, clientInfo *types.ClientInfo) error {
	username, err := client.LoginInput("Username > ", reader)
	if err != nil {
		return fmt.Errorf("error reading username: %v", err)

	}

	password, err := client.LoginInput("Password > ", reader)
	if err != nil {
		return fmt.Errorf("error reading password: %v", err)
	}

	if username == "" || password == "" {
		return fmt.Errorf("username and password cannot be empty")
	}

	clientInfo.Username = username

	err = client.Login(clientInfo, password)
	if err != nil {
		return err
	}

	return nil
}

func handleSignup(reader *bufio.Reader, clientInfo *types.ClientInfo) error {
	username, err := client.LoginInput("Username > ", reader)
	if err != nil {
		return fmt.Errorf("error reading username: %v", err)
	}

	password, err := client.LoginInput("Password > ", reader)
	if err != nil {
		return fmt.Errorf("error reading username: %v", err)
	}

	if username == "" || password == "" {
		return fmt.Errorf("username and password cannot be empty")
	}

	// Create a new UUID
	clientInfo.UserID = uuid.New()
	clientInfo.Username = username
	//TODO: Handle Keyloading here?

	encKey, ok1 := clientInfo.Keys["EncryptionPublicKey"]
	sigKey, ok2 := clientInfo.Keys["SigningPublicKey"]

	if !ok1 || !ok2 {
		return fmt.Errorf("missing required public keys for signup")
	}

	err = client.Signup(clientInfo, password, encKey, sigKey)
	if err != nil {
		return fmt.Errorf("error connecting: %v", err)
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

func launchREPL(c *types.ClientInfo) error {
	inputReader := bufio.NewReader(os.Stdin)

	fmt.Println("Type /login to log into the Strike Messaging service")
	fmt.Println("Type /signup to signup to the Strike Messaging service")
	fmt.Println("Type /exit to quit.")

	loggedin := false

	for {
		if !loggedin {
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
				err := handleLogin(inputReader, c)
				if err != nil {
					fmt.Printf("login error: %v\n", err)
					continue
				}
				loggedin = true
			case "/signup":
				if err := handleSignup(inputReader, c); err != nil {
					fmt.Printf("signup error: %v\n", err)
					continue
				}
				loggedin = true
			case "/exit":
				fmt.Println("Strike Client shutting down")
				return nil
			default:
				fmt.Printf("Unknown command: %s\nTry /help for available commands.", input)
			}

		} else {
			go func() {
				if err := client.RegisterStatus(c); err != nil {
					fmt.Printf("error connecting stream: %v\n", err)
					return
				}
			}()

			fmt.Printf("Welcome back %s!\n", c.Username)

			if err := client.MShell(c); err != nil {
				return err
			}
		}
	}
}
