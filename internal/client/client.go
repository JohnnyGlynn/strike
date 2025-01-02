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

	"google.golang.org/protobuf/types/known/timestamppb"
)

type Config struct {
	ServerHost               string `json:"server_host" yaml:"server_host"`
	Username                 string `json:"username" yaml:"username"`
	SigningPrivateKeyPath    string `json:"private_signing_key_path" yaml:"private_singing_key_path"`
	SigningPublicKeyPath     string `json:"public_signing_key_path" yaml:"public_signing_key_path"`
	EncryptionPrivateKeyPath string `json:"private_encryption_key_path" yaml:"private_encryption_key_path"`
	EncryptionPublicKeyPath  string `json:"public_encryption_key_path" yaml:"public_encryption_key_path"`
}

func LoadConfigEnv() *Config {
	return &Config{
		ServerHost:               os.Getenv("SERVER_HOST"),
		Username:                 os.Getenv("USERNAME"),
		SigningPrivateKeyPath:    os.Getenv("PRIVATE_SIGNING_KEY_PATH"),
		SigningPublicKeyPath:     os.Getenv("PUBLIC_SIGNING_KEY_PATH"),
		EncryptionPrivateKeyPath: os.Getenv("PRIVATE_ENCRYPTION_KEY_PATH"),
		EncryptionPublicKeyPath:  os.Getenv("PUBLIC_ENCRYPTION_KEY_PATH"),
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
	if c.SigningPrivateKeyPath == "" {
		return fmt.Errorf("private_signing_key_path is required")
	}
	if c.SigningPublicKeyPath == "" {
		return fmt.Errorf("public_signing_key_path is required")
	}
	if c.EncryptionPrivateKeyPath == "" {
		return fmt.Errorf("private_encryption_key_path is required")
	}
	if c.EncryptionPublicKeyPath == "" {
		return fmt.Errorf("public_encryption_key_path is required")
	}
	return nil
}

func AutoChat(c pb.StrikeClient, uname string, pubkey []byte) error {

	newChat := pb.Chat{
		Name:    "endpoint0",
		Message: "Hello from client0",
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		for {
			now := time.Now()
			timestamp := timestamppb.New(now)

			newEnvelope := pb.Envelope{
				SenderPublicKey: pubkey,
				SentAt:          timestamp,
				Chat:            &newChat,
			}

			stamp, err := c.SendMessages(ctx, &newEnvelope)
			if err != nil {
				log.Fatalf("SendMessages Failed: %v", err)
			}

			fmt.Println("Stamp: ", stamp.KeyUsed)

			time.Sleep(5 * time.Second)
		}
	}()

	stream, err := c.GetMessages(ctx, &newChat)
	if err != nil {
		log.Fatalf("GetMessages Failed: %v", err)
		return err
	}

	for {
		messageStream, err := stream.Recv()
		if err == io.EOF {
			log.Printf("Recieve Failed EOF: %v", err)
			log.Printf("Awaiting Messages")
			continue
		}
		if err != nil {
			log.Fatalf("Recieve Failed: %v", err)
			return err
		}
		fmt.Printf("Chat Name: %s\n", messageStream.Chat.Name)
		fmt.Printf("Message: %s\n", messageStream.Chat.Message)
	}

}

func RegisterClient(c pb.StrikeClient, uname string, pubkey []byte) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	initClient := pb.ClientInit{
		Uname:     uname,
		PublicKey: pubkey,
	}

	stamp, err := c.KeyHandshake(ctx, &initClient)
	if err != nil {
		log.Fatalf("KeyHandshake Failed: %v", err)
		return err
	}

	// TODO: Print actual SenderPublicKey
	fmt.Printf("Stamp: %v\n", stamp.KeyUsed)
	return nil
}

func Login(c pb.StrikeClient, uname string, pubkey []byte) error {

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	loginClient := pb.ClientLogin{
		Uname:     uname,
		PublicKey: pubkey,
	}

	stamp, err := c.Login(ctx, &loginClient)
	if err != nil {
		log.Fatalf("Login Failed: %v", err)
		return err
	}

	// TODO: Print actual SenderPublicKey
	log.Printf("Stamp: %v\n", stamp.KeyUsed)
	return nil
}
