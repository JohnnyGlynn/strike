package main

import (
	"fmt"
	"log"
	"net"

	"github.com/JohnnyGlynn/strike/internal/server"
	pb "github.com/JohnnyGlynn/strike/msgdef/message"

	grpc "google.golang.org/grpc"
)

func main() {
	fmt.Println("Strike Server")

	lis, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	var opts []grpc.ServerOption

	srvr := grpc.NewServer(opts...)
	pb.RegisterStrikeServer(srvr, server.InitServer())

	srvr.Serve(lis)

	err = srvr.Serve(lis)
	if err != nil {
		fmt.Printf("Error listening: %v\n", err)
	}

}
