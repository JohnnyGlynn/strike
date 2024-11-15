package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	pb "github.com/JohnnyGlynn/strike/msgdef/message"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// func printMessages(ctx context.Context, client pb.StrikeClient, chat *pb.Chat) {
// 	stream, err := client.GetMessages(ctx, chat)
// 	if err != nil {
// 		fmt.Println("broken")
// 		fmt.Println(err)
// 	}

// 	for {
// 		envelope, err := stream.Recv()
// 		if err == io.EOF {
// 			break
// 		}
// 		if err != nil {
// 			println("broken stream")
// 		}
// 		fmt.Printf("\nChat:%v\n\n\nSenderKey:%v\nHash Time:%v\nTime:%v",
// 			envelope.Chat.Name, envelope.SenderPublicKey, envelope.HashTime, envelope.Time)
// 	}
// }

// func sendMessages(ctx context.Context, client pb.StrikeClient, envelope *pb.Envelope) {
// 	stamp, err := client.SendMessages(ctx, envelope)
// 	if err != nil {
// 		// fmt.Printf("broken2 fmt: %v\n", err)
// 		log.Fatalf("broken2: %v", err)
// 	}

// 	fmt.Printf("New Message sent: %v\n", stamp)

// }

func main() {
	fmt.Println("Strike client")

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
