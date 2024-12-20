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

func registerClient(pubkeypath string) {

	//TODO create a client once
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))

	conn, err := grpc.NewClient("localhost:8080", opts...)
	if err != nil {
		log.Fatalf("fail to dial: %v", err)
	}
	defer conn.Close()

	client := pb.NewStrikeClient(conn)

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

	stamp, err := client.KeyHandshake(ctx, &initClient)
	if err != nil {
		log.Fatalf("KeyHandshake Failed: %v", err)
	}

	// TODO: Print actual SenderPublicKey
	fmt.Printf("Stamp: %v\n", stamp.KeyUsed)
}

func login(uname string, pubkeypath string) {

	//TODO create a client once
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))

	conn, err := grpc.NewClient("localhost:8080", opts...)
	if err != nil {
		log.Fatalf("fail to dial: %v", err)
	}
	defer conn.Close()

	client := pb.NewStrikeClient(conn)

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

	stamp, err := client.Login(ctx, &loginClient)
	if err != nil {
		log.Fatalf("Login Failed: %v", err)
	}

	// TODO: Print actual SenderPublicKey
	fmt.Printf("Stamp: %v\n", stamp.KeyUsed)
}

func main() {
	fmt.Println("Strike client")

	//check if its the first startup then create a userfile and generate keys
	_, err := os.Stat("./cfg/userfile")
	if os.IsNotExist(err) {
		fmt.Println("First time setup")

		//create config directory
		direrr := os.Mkdir("./cfg", 0755)
		if direrr != nil {
			fmt.Println("Error creating directory:", direrr)
			return
		}

		//create userfile
		_, ferr := os.Create("./cfg/userfile")
		if ferr != nil {
			fmt.Println("Error creating userfile:", ferr)
			os.Exit(1)
		}
		//key generation
		pubpath := keys.Keygen()
		registerClient(pubpath)
		login("client0", pubpath)
		autoChat()

	} else {
		autoChat()
	}

}
