package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/JohnnyGlynn/strike/internal/config"
	"github.com/JohnnyGlynn/strike/internal/keys"
	"github.com/JohnnyGlynn/strike/internal/server"
	// fedpb "github.com/JohnnyGlynn/strike/msgdef/federation"
	// pb "github.com/JohnnyGlynn/strike/msgdef/message"

	"google.golang.org/grpc/credentials"
)

func main() {
	fmt.Println("Strike Server")

	// Avoid shadowing
	var serverCfg config.ServerConfig
	var err error

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		s := <-sigCh
		log.Printf("%s: initiating graceful shutdown", s)
		cancel()
	}()

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
			fmt.Printf("Failed to load server config: %v", err)
			return
		}

		if err = serverCfg.ValidateConfig(); err != nil {
			fmt.Printf("Invalid Server config: %v", err)
			return
		}

	} else if !*keygen {
		log.Println("Loading Config from Envrionment Variables")

		serverCfg = *config.LoadServerConfigEnv()

		if err = serverCfg.ValidateEnv(); err != nil {
			fmt.Printf("Invalid Server config: %v", err)
			return
		}
	}

	log.Printf("Loaded Server Config: %+v", serverCfg)

	// pgConfig, err := pgxpool.ParseConfig(serverCfg.DBConnectionString)
	// if err != nil {
	// 	fmt.Printf("Config parsing failed: %v", err)
	// 	return
	// }

	// pool, err := pgxpool.NewWithConfig(ctx, pgConfig)
	// if err != nil {
	// 	fmt.Printf("DB pool connection failed: %v", err)
	// 	return
	// }
	// defer pool.Close()

	// statements, err := server.PrepareStatements(ctx, pool)
	// if err != nil {
	// 	fmt.Printf("Failed to prepare statements: %v", err)
	// 	return
	// }

	// Load TLS credentials for Strike gRPC server
	creds, err := credentials.NewServerTLSFromFile(serverCfg.CertificatePath, serverCfg.SigningPrivateKeyPath)
	if err != nil {
		log.Fatalf("failed to load TLS credentials: %v", err)
	}

	// Initialize bootstrap
	bootstrap := server.InitBootstrap(serverCfg)

	// Initialize DB
	if err := bootstrap.InitDb(ctx); err != nil {
		log.Fatalf("DB initialization failed: %v", err)
	}

	// Initialize Strike server
	if err := bootstrap.InitStrikeServer(creds); err != nil {
		log.Fatalf("Strike server initialization failed: %v", err)
	}

	// Initialize Federation server
	if err := bootstrap.InitFederation(); err != nil {
		log.Fatalf("Federation server initialization failed: %v", err)
	}

	// Start servers and peer connections
	if err := bootstrap.Start(ctx); err != nil {
		log.Fatalf("bootstrap start failed: %v", err)
	}

	// Wait for shutdown signal
	<-ctx.Done()
	log.Println("Shutdown signal received")

	bootstrap.Stop(ctx)
}
