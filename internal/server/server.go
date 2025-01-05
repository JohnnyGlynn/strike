package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/JohnnyGlynn/strike/internal/db"
	pb "github.com/JohnnyGlynn/strike/msgdef/message"
)

type StrikeServer struct {
	pb.UnimplementedStrikeServer
	Env         []*pb.Envelope
	DBpool      *pgxpool.Pool
	PStatements *db.PreparedStatements
	// mu sync.Mutex
}

type Config struct {
	ServerName            string `json:"server_name" yaml:"server_name"`
	SigningPrivateKeyPath string `json:"private_signing_key_path" yaml:"private_singing_key_path"`
	SigningPublicKeyPath  string `json:"public_signing_key_path" yaml:"public_signing_key_path"`
	CertificatePath       string `json:"certificate_path" yaml:"certificate_path"`
}

func LoadConfigEnv() *Config {
	return &Config{
		ServerName:            os.Getenv("SERVER_NAME"),
		SigningPrivateKeyPath: os.Getenv("PRIVATE_SIGNING_KEY_PATH"),
		SigningPublicKeyPath:  os.Getenv("PUBLIC_SIGNING_KEY_PATH"),
		CertificatePath:       os.Getenv("SERVER_CERT_PATH"),
	}
}

// TODO: Fine for binary testing but probably just going to provide env vars to container
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
	if c.ServerName == "" {
		return fmt.Errorf("server_host is required")
	}
	if c.SigningPrivateKeyPath == "" {
		return fmt.Errorf("private_signing_key_path is required")
	}
	if c.SigningPublicKeyPath == "" {
		return fmt.Errorf("public_signing_key_path is required")
	}
	if c.CertificatePath == "" {
		return fmt.Errorf("private_encryption_key_path is required")
	}
	return nil
}

func (s *StrikeServer) GetMessages(chat *pb.Chat, stream pb.Strike_GetMessagesServer) error {
	fmt.Println("Streaming messages for chat: endpoint0")

	messageChannel := make(chan *pb.Envelope)

	go func() {
		for {
			//TODO: DB query? Load from DB into cache?
			messages, err := s.fetchMessages()
			if err != nil {
				fmt.Printf("Error Fetching messages")
			}

			for _, msg := range messages {
				messageChannel <- msg
			}

			time.Sleep(1 * time.Second) // Slow down
		}
	}()

	for {
		select {
		case <-stream.Context().Done():
			// TODO: Graceful Disconnect/Shutdown
			fmt.Println("Client disconnected.")
			return nil
		case msg := <-messageChannel:
			// Send message on stream
			if err := stream.Send(msg); err != nil {
				fmt.Printf("Failed to stream message: %v\n", err)
				return err
			}
		case <-time.After(1 * time.Second):
			// TODO: Keep-alive/Heartbeat
		}
	}
}

func (s *StrikeServer) SendMessages(ctx context.Context, envelope *pb.Envelope) (*pb.Stamp, error) {
	// fmt.Printf("Received message: %s\n", envelope)
	err := s.storeMessage(envelope)
	if err != nil {
		fmt.Printf("Error storing message")
		return nil, err
	}
	return &pb.Stamp{KeyUsed: envelope.SenderPublicKey}, nil
}

func (s *StrikeServer) Login(ctx context.Context, clientLogin *pb.ClientLogin) (*pb.Stamp, error) {
	fmt.Println("Logging in...")

	var exists bool

	err := s.DBpool.QueryRow(ctx, s.PStatements.LoginUser, clientLogin.Uname, clientLogin.PublicKey).Scan(&exists)
	if err != nil {
		if pgerr, ok := err.(*pgconn.PgError); ok && pgerr.Code == "no-data-found" {
			log.Fatalf("Unable to login: %v", err)
			return nil, nil
		}
		log.Fatalf("An Error occured while logging in: %v", err)
		return nil, nil
	}

	fmt.Println("Login Successful...")

	return &pb.Stamp{KeyUsed: clientLogin.PublicKey}, nil
}

func (s *StrikeServer) KeyHandshake(ctx context.Context, clientinit *pb.ClientInit) (*pb.Stamp, error) {
	fmt.Printf("New User signup: %s\n", clientinit.Uname)
	fmt.Printf("%v's Public Key: %v\n", clientinit.Uname, clientinit.PublicKey)

	_, err := s.DBpool.Exec(ctx, s.PStatements.CreateUser, clientinit.Uname, clientinit.PublicKey)
	if err != nil {
		log.Fatalf("Insert failed: %v", err)
	}

	return &pb.Stamp{KeyUsed: clientinit.PublicKey}, nil
}

func (s *StrikeServer) storeMessage(envelope *pb.Envelope) error {
	//TODO: DB operations here - slice for now
	s.Env = append(s.Env, envelope)
	return nil
}

// TODO: RecieveMessages instead of GetMessages, then getMessages
func (s *StrikeServer) fetchMessages() ([]*pb.Envelope, error) {
	var result []*pb.Envelope
	// TODO: make channels for chats
	for _, envelope := range s.Env {
		if envelope.Chat.Name == "endpoint0" {
			result = append(result, envelope)
		}
	}
	return result, nil
}
