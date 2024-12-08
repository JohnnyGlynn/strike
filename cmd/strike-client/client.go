package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/JohnnyGlynn/strike/cmd/keys"
	pb "github.com/JohnnyGlynn/strike/msgdef/message"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func autoChat() {
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))

	conn, err := grpc.NewClient("localhost:8080", opts...)
	if err != nil {
		log.Fatalf("fail to dial: %v", err)
	}
	defer conn.Close()

	client := pb.NewStrikeClient(conn)

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
		stamp, err := client.SendMessages(ctx, &newEnvelope)
		if err != nil {
			log.Fatalf("SendMessages Failed: %v", err)
		}

		// TODO: Print actual SenderPublicKey
		fmt.Printf("Stamp: %v\n", stamp.KeyUsed)

		stream, err := client.GetMessages(ctx, &newChat)
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

func main() {
	fmt.Println("Strike client")

	//check if its the first startup then create a userfile and generate keys
	_, err := os.Stat("userfile")
	if os.IsNotExist(err) {
		//create userfile
		_, err := os.Create("userfile")
		if err != nil {
			fmt.Println("Error creating userfile:", err)
			os.Exit(1)
		}
		//key generation
		keys.Keygen()
		autoChat()

	} else {
		autoChat()
	}

}
