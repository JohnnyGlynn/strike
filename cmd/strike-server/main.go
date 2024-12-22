package main

import (
	"context"
	"fmt"
	"log"
	"net"

	"github.com/JohnnyGlynn/strike/internal/server"
	pb "github.com/JohnnyGlynn/strike/msgdef/message"

	"github.com/jackc/pgx/v5/pgxpool"
	grpc "google.golang.org/grpc"
)

func main() {
	fmt.Println("Strike Server")

	lis, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	var opts []grpc.ServerOption

	config, err := pgxpool.ParseConfig("postgres://strikeadmin:plaintextisbad@strike_db:5432/strike")
	if err != nil {
		log.Fatalf("Config parsing failed: %v", err)
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		log.Fatalf("DB pool connection failed: %v", err)
	}
	defer pool.Close()

	srvr := grpc.NewServer(opts...)
	pb.RegisterStrikeServer(srvr, &server.StrikeServer{DBpool: pool})

	srvr.Serve(lis)

	err = srvr.Serve(lis)
	if err != nil {
		fmt.Printf("Error listening: %v\n", err)
	}

}
