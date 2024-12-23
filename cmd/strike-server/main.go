package main

import (
	"context"
	"fmt"
	"log"
	"net"

	"github.com/JohnnyGlynn/strike/internal/db"
	"github.com/JohnnyGlynn/strike/internal/server"
	pb "github.com/JohnnyGlynn/strike/msgdef/message"

	"github.com/jackc/pgx/v5/pgxpool"
	grpc "google.golang.org/grpc"
)

func main() {
	fmt.Println("Strike Server")
	ctx := context.Background()

	config, err := pgxpool.ParseConfig("postgres://strikeadmin:plaintextisbad@strike_db:5432/strike")
	if err != nil {
		log.Fatalf("Config parsing failed: %v", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		log.Fatalf("DB pool connection failed: %v", err)
	}
	defer pool.Close()

	statements, err := db.PrepareStatements(ctx, pool)
	if err != nil {
		log.Fatalf("Failed to prepare statements: %v", err)
	}

	strikeServerConfig := &server.StrikeServer{
		DBpool:      pool,
		PStatements: statements,
	}

	//GRPC server prep
	lis, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	var opts []grpc.ServerOption

	srvr := grpc.NewServer(opts...)
	pb.RegisterStrikeServer(srvr, strikeServerConfig)

	err = srvr.Serve(lis)
	if err != nil {
		fmt.Printf("Error listening: %v\n", err)
	}

}
