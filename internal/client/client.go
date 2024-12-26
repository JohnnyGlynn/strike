package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	pb "github.com/JohnnyGlynn/strike/msgdef/message"
)

type Config struct {
	ServerHost     string `json:"server_host" yaml:"server_host"`
	Username       string `json:"username" yaml:"username"`
	PrivateKeyPath string `json:"private_key_path" yaml:"private_key_path"`
	PublicKeyPath  string `json:"public_key_path" yaml:"public_key_path"`
}

func LoadConfigEnv() *Config {
	return &Config{
		ServerHost:     os.Getenv("SERVER_HOST"),
		Username:       os.Getenv("USERNAME"),
		PrivateKeyPath: os.Getenv("PRIVATE_KEY_PATH"),
		PublicKeyPath:  os.Getenv("PUBLIC_KEY_PATH"),
	}
}

func LoadConfigFile(filePath string) (*Config, error) {
	configFile, err := os.ReadFile(filePath)
	if err != nil {
		log.Fatalf("Failed to read config file: %v", err)
		return nil, err
	}

	var config Config

	err = json.Unmarshal(configFile, &config)
	if err != nil {
		log.Fatalf("Unmarshall Failed: %v", err)
		return nil, err
	}

	return &config, nil
}

// TODO: Handle this better
func (c *Config) ValidateConfig() error {
	if c.ServerHost == "" {
		return fmt.Errorf("server_host is required")
	}
	if c.Username == "" {
		return fmt.Errorf("username is required")
	}
	if c.PrivateKeyPath == "" {
		return fmt.Errorf("private_key_path is required")
	}
	if c.PublicKeyPath == "" {
		return fmt.Errorf("public_key_path is required")
	}
	return nil
}

func AutoChat(c pb.StrikeClient) {
	newChat := pb.Chat{
		Name:    "endpoint0",
		Message: "Hello from client0",
	}

	newEnvelope := pb.Envelope{
		SenderPublicKey: 123,
		HashTime:        0010,
		Time:            0010,
		Chat:            &newChat,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for {
		stamp, err := c.SendMessages(ctx, &newEnvelope)
		if err != nil {
			log.Fatalf("SendMessages Failed: %v", err)
		}

		// TODO: Print actual SenderPublicKey
		fmt.Printf("Stamp: %v\n", stamp.KeyUsed)

		stream, err := c.GetMessages(ctx, &newChat)
		if err != nil {
			log.Fatalf("GetMessages Failed: %v", err)
		}

		for {
			messageStream, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Fatalf("Recieve Failed: %v", err)
			}
			fmt.Printf("Chat Name: %s\n", messageStream.Chat.Name)
			fmt.Printf("Message: %s\n", messageStream.Chat.Message)
		}

		//Slow down
		time.Sleep(5 * time.Second)
	}
}

func RegisterClient(c pb.StrikeClient, pubkeypath string) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	//TODO: More robust than this
	publickeyfile, err := os.Open(pubkeypath)
	if err != nil {
		fmt.Println("Error public key file:", err)
		return
	}
	defer publickeyfile.Close()

	publickey, err := io.ReadAll(publickeyfile)
	if err != nil {
		fmt.Println("Error reading public key:", err)
		return
	}

	initClient := pb.ClientInit{
		Uname:     "client0",
		PublicKey: publickey,
	}

	stamp, err := c.KeyHandshake(ctx, &initClient)
	if err != nil {
		log.Fatalf("KeyHandshake Failed: %v", err)
	}

	// TODO: Print actual SenderPublicKey
	fmt.Printf("Stamp: %v\n", stamp.KeyUsed)
}

func Login(c pb.StrikeClient, uname string, pubkeypath string) {

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	//TODO: More robust than this
	publickeyfile, err := os.Open(pubkeypath)
	if err != nil {
		fmt.Println("Error public key file:", err)
		return
	}
	defer publickeyfile.Close()

	publickey, err := io.ReadAll(publickeyfile)
	if err != nil {
		fmt.Println("Error reading public key:", err)
		return
	}

	loginClient := pb.ClientLogin{
		Uname:     uname,
		PublicKey: publickey,
	}

	stamp, err := c.Login(ctx, &loginClient)
	if err != nil {
		log.Fatalf("Login Failed: %v", err)
	}

	// TODO: Print actual SenderPublicKey
	fmt.Printf("Stamp: %v\n", stamp.KeyUsed)
}
