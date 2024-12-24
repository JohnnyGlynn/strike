package main

import (
	"fmt"
	"log"
	"os"

	"github.com/JohnnyGlynn/strike/internal/client"
	"github.com/JohnnyGlynn/strike/internal/keys"

	pb "github.com/JohnnyGlynn/strike/msgdef/message"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	fmt.Println("Strike client")

	var opts []grpc.DialOption
	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))

	conn, err := grpc.NewClient("strike_server:8080", opts...)
	if err != nil {
		log.Fatalf("fail to dial: %v", err)
	}
	defer conn.Close()

	newClient := pb.NewStrikeClient(conn)

	//check if its the first startup then create a userfile and generate keys
	_, err = os.Stat("./cfg/userfile")
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

		client.RegisterClient(newClient, pubpath)
		client.Login(newClient, "client0", pubpath)
		client.AutoChat(newClient)

	} else {
		client.AutoChat(newClient)
	}

}
