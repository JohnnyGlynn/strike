package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/JohnnyGlynn/strike/internal/config"
	"github.com/JohnnyGlynn/strike/internal/db"
	"github.com/JohnnyGlynn/strike/internal/keys"
	"github.com/JohnnyGlynn/strike/internal/server"
	pb "github.com/JohnnyGlynn/strike/msgdef/message"
	"github.com/jackc/pgx/v5/pgxpool"

	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func main() {
	fmt.Println("Strike Server")
	ctx := context.Background()

	// Avoid shadowing
	var serverCfg config.ServerConfig
	var err error

	// TODO: Refactor, replicated from client
	configFilePath := flag.String("config", "", "Path to configuration JSON file")
	keygen := flag.Bool("keygen", false, "Launch Strike Server Key generation, creating keypair and certificate")
	flag.Parse()

	if *keygen {
		err := keys.GenerateServerKeysAndCert()
		if err != nil {
			fmt.Printf("error generating server signing keys and certificate: %v\n", err)
			return
		}
		os.Exit(0)
	}

	if *configFilePath != "" && !*keygen {
		log.Println("Loading Config from File")

		serverCfg, err = config.LoadConfigFile[config.ServerConfig](*configFilePath)
		if err != nil {
			log.Fatalf("Failed to load server config: %v", err)
		}

		if err = serverCfg.ValidateConfig(); err != nil {
			log.Fatalf("Invalid Server config: %v", err)
		}

	} else if !*keygen {
		log.Println("Loading Config from Envrionment Variables")

		serverCfg = *config.LoadServerConfigEnv()

		if err = serverCfg.ValidateEnv(); err != nil {
			log.Fatalf("Invalid Server config: %v", err)
		}
	}

	// +v to print struct fields too
	log.Printf("Loaded Server Config: %+v", serverCfg)

	// PostgreSQL setup
	pgConfig, err := pgxpool.ParseConfig("postgres://strikeadmin:plaintextisbad@strike_db:5432/strike")
	if err != nil {
		log.Fatalf("Config parsing failed: %v", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, pgConfig)
	if err != nil {
		log.Fatalf("DB pool connection failed: %v", err)
	}
	defer pool.Close()

	statements, err := db.PrepareStatements(ctx, pool)
	if err != nil {
		log.Fatalf("Failed to prepare statements: %v", err)
	}

	serverCreds, err := credentials.NewServerTLSFromFile(serverCfg.CertificatePath, serverCfg.SigningPrivateKeyPath)
	if err != nil {
		log.Fatalf("Failed to load TLS credentials: %v", err)
	}

	log.Println("Loaded TLS credentials")

	strikeServerConfig := &server.StrikeServer{
		DBpool:      pool,
		PStatements: statements,
	}

	// GRPC server prep
	lis, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	opts := []grpc.ServerOption{
		grpc.Creds(serverCreds),
	}

	srvr := grpc.NewServer(opts...)
	pb.RegisterStrikeServer(srvr, strikeServerConfig)

	err = srvr.Serve(lis)
	if err != nil {
		fmt.Printf("Error listening: %v\n", err)
	}
}
