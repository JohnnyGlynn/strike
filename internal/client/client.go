package client

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	pb "github.com/JohnnyGlynn/strike/msgdef/message"
)

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
