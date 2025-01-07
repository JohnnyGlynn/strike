package client

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	pb "github.com/JohnnyGlynn/strike/msgdef/message"

	"google.golang.org/protobuf/types/known/timestamppb"
)

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
