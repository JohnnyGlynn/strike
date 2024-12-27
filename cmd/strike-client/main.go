package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/JohnnyGlynn/strike/internal/client"

	pb "github.com/JohnnyGlynn/strike/msgdef/message"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	fmt.Println("Strike client")

	configFilePath := flag.String("config", "", "Path to configuration JSON file")
	flag.Parse()

	//Avoid shadowing
	var config *client.Config
	var err error

	/*
		Flag check - Provide a config file, otherwise look for env vars
		Average user who just wants to connect to a server can run binary+json file,
		meanwhile running a server you can have a client contianer present with env vars provided to pod
	*/
	if *configFilePath != "" {
		log.Println("Loading Config from File")
		config, err = client.LoadConfigFile(*configFilePath)
		if err != nil {
			log.Fatalf("Failed to load client config: %v", err)
		}
		//TODO: Looks gross
		if err := config.ValidateConfig(); err != nil {
			log.Fatalf("Invalid client config: %v", err)
		}
	} else {
		log.Println("Loading Config from Envrionment Variables")
		config = client.LoadConfigEnv()
		if err := config.ValidateConfig(); err != nil {
			log.Fatalf("Invalid client config: %v", err)
		}
	}

	// +v to print struct fields too
	log.Printf("Loaded client Config: %+v", config)

	var opts []grpc.DialOption
	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))

	conn, err := grpc.NewClient(config.ServerHost, opts...)
	if err != nil {
		log.Fatalf("fail to dial: %v", err)
	}
	defer conn.Close()

	newClient := pb.NewStrikeClient(conn)

	// client.RegisterClient(newClient, config.PublicKeyPath)
	client.Login(newClient, config.Username, config.PublicKeyPath)
	client.AutoChat(newClient)
}
