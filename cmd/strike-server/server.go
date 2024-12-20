package strike_server

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	pb "github.com/JohnnyGlynn/strike/msgdef/message"
)

type strikeServer struct {
	pb.UnimplementedStrikeServer
	Env []*pb.Envelope
	// mu sync.Mutex
}

func (s *strikeServer) GetMessages(chat *pb.Chat, stream pb.Strike_GetMessagesServer) error {
	for _, envelope := range s.Env {
		if envelope.Chat.Name == "endpoint0" {
			if err := stream.Send(envelope); err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *strikeServer) SendMessages(ctx context.Context, envelope *pb.Envelope) (*pb.Stamp, error) {
	fmt.Printf("Received message: %s\n", envelope)
	return &pb.Stamp{KeyUsed: envelope.SenderPublicKey}, nil
}

func (s *strikeServer) Login(ctx context.Context, clientLogin *pb.ClientLogin) (*pb.Stamp, error) {

	fmt.Println("Logging in...")

	//TODO: Secrets for connection string
	config, err := pgxpool.ParseConfig("postgres://strikeadmin:plaintextisbad@localhost:5432/strike")
	if err != nil {
		log.Fatalf("Config parsing failed: %v", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		log.Fatalf("DB connection failed: %v", err)
	}
	defer pool.Close()

	var exists bool

	//TODO: Prep these statements elsewhere and call as needed
	err = pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM userkeys WHERE uname = $1 AND publickey = $2)", clientLogin.Uname, clientLogin.PublicKey).Scan(&exists)
	if err != nil {
		if pgerr, ok := err.(*pgconn.PgError); ok && pgerr.Code == "no-data-found" {
			log.Fatalf("Unable to login: %v", err)
			return nil, nil
		}
		log.Fatalf("An Error occured while logging in: %v", err)
		return nil, nil
	}

	fmt.Println("Login Successful...")

	//TODO: Change message type to support keys not ints
	var keyAsInt int32
	binary.Read(bytes.NewReader(clientLogin.PublicKey), binary.BigEndian, &keyAsInt)

	return &pb.Stamp{KeyUsed: keyAsInt}, nil
}

func (s *strikeServer) KeyHandshake(ctx context.Context, clientinit *pb.ClientInit) (*pb.Stamp, error) {
	//TODO: Create a "user" in the DB
	fmt.Printf("New User signup: %s\n", clientinit.Uname)
	fmt.Printf("%v's Public Key: %v\n", clientinit.Uname, clientinit.PublicKey)

	//TODO: Secrets for connection string
	config, err := pgxpool.ParseConfig("postgres://strikeadmin:plaintextisbad@localhost:5432/strike")
	if err != nil {
		log.Fatalf("Config parsing failed: %v", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		log.Fatalf("DB connection failed: %v", err)
	}
	defer pool.Close()

	//TODO: Prep these statements elsewhere and call as needed
	_, err = pool.Exec(ctx, "INSERT INTO userkeys (uname, publickey) VALUES ($1, $2)", clientinit.Uname, clientinit.PublicKey)
	if err != nil {
		log.Fatalf("Insert failed: %v", err)
	}

	//TODO: Change message type to support keys not ints
	var keyAsInt int32
	binary.Read(bytes.NewReader(clientinit.PublicKey), binary.BigEndian, &keyAsInt)

	return &pb.Stamp{KeyUsed: keyAsInt}, nil
}

func InitServer() *strikeServer {
	s := strikeServer{}
	return &s
}
