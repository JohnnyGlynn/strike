package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/JohnnyGlynn/strike/internal/config"
	"github.com/JohnnyGlynn/strike/internal/keys"
	"github.com/JohnnyGlynn/strike/internal/server"
	// fedpb "github.com/JohnnyGlynn/strike/msgdef/federation"
	// pb "github.com/JohnnyGlynn/strike/msgdef/message"

	"google.golang.org/grpc/credentials"
)

func main() {
	// stderr, not stdout — --export-peer's output is meant to be redirected
	// straight into a file (e.g. `--export-peer ... > peer-card.yaml`), and a
	// banner on stdout would corrupt that.
	fmt.Fprintln(os.Stderr, "Strike Server")

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
	genCA := flag.Bool("gen-ca", false, "Generate a Strike Federation CA keypair and certificate")
	genFed := flag.Bool("gen-federation", false, "Generate federation.yaml from peer key directories (use with --peer flags and --output)")
	outputPath := flag.String("output", "./config/server/federation.yaml", "Output path for generated federation config")
	keydir := flag.String("keydir", ".", "Output directory for generated keys and certificate")
	serverName := flag.String("name", "", "Server name for identity file (used with --keygen)")
	caCertPath := flag.String("ca-cert", "", "Path to CA certificate for signing server cert")
	caKeyPath := flag.String("ca-key", "", "Path to CA private key for signing server cert")
	sanFlag := flag.String("san", "", "Comma-separated hostnames/IPs this server will be reachable at (used with --keygen), e.g. the public IP or DDNS name a friend will connect to")
	exportPeer := flag.Bool("export-peer", false, "Print this server's federation peer entry (name/addr/pubkey) to share with a friend running their own Strike server")
	addPeer := flag.Bool("add-peer", false, "Read a peer entry (as produced by --export-peer) from stdin and add it to --output")
	addrFlag := flag.String("addr", "", "Address this server is reachable at for federation, e.g. host:9090 (used with --export-peer)")
	flag.Parse()

	var extraSANs []string
	if *sanFlag != "" {
		extraSANs = strings.Split(*sanFlag, ",")
	}

	if *exportPeer {
		if *serverName == "" || *addrFlag == "" {
			fmt.Println("usage: --export-peer --name=<your-name> --addr=<host:9090> [--keydir=.]")
			return
		}

		pubKeyPath := filepath.Join(*keydir, "strike_server_public.pem")
		peerBlock, err := server.ExportPeerBlock(*serverName, *addrFlag, pubKeyPath)
		if err != nil {
			fmt.Printf("error exporting peer entry: %v\n", err)
			return
		}

		fmt.Println("# Send this to a friend running their own Strike server.")
		fmt.Printf("# They add it with: ./strike-server --add-peer --output=<their federation.yaml> < peer-%s.yaml\n", *serverName)
		fmt.Print(peerBlock)
		os.Exit(0)
	}

	if *addPeer {
		card, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Printf("failed to read peer entry from stdin: %v\n", err)
			return
		}

		if err := server.AddPeerToFile(*outputPath, card); err != nil {
			fmt.Printf("error adding peer: %v\n", err)
			return
		}

		fmt.Printf("Peer added to %s\n", *outputPath)
		os.Exit(0)
	}

	if *genFed {
		// Remaining args are peer specs using comma delimiter: name,addr,keydir
		args := flag.Args()
		if len(args) == 0 {
			fmt.Println("usage: --gen-federation name,addr,keydir [name,addr,keydir ...]")
			return
		}

		var peers []keys.PeerEntry
		for _, arg := range args {
			parts := strings.SplitN(arg, ",", 3)
			if len(parts) != 3 {
				fmt.Printf("invalid peer spec %q — expected name,addr,keydir\n", arg)
				return
			}
			peers = append(peers, keys.PeerEntry{
				Name:   parts[0],
				Addr:   parts[1],
				KeyDir: parts[2],
			})
		}

		if err := keys.GenerateFederationConfig(peers, *outputPath); err != nil {
			fmt.Printf("error generating federation config: %v\n", err)
			return
		}
		os.Exit(0)
	}

	if *genCA {
		err := keys.GenerateCA(*keydir)
		if err != nil {
			fmt.Printf("error generating CA: %v\n", err)
			return
		}
		os.Exit(0)
	}

	if *keygen {
		if *caCertPath != "" && *caKeyPath != "" {
			caCert, caKey, err := keys.LoadCA(*caCertPath, *caKeyPath)
			if err != nil {
				fmt.Printf("error loading CA: %v\n", err)
				return
			}
			err = keys.GenerateServerKeysAndCertWithCA(*keydir, caCert, caKey, extraSANs)
			if err != nil {
				fmt.Printf("error generating server signing keys and certificate: %v\n", err)
				return
			}
		} else {
			err = keys.GenerateServerKeysAndCertWithCA(*keydir, nil, nil, extraSANs)
			if err != nil {
				fmt.Printf("error generating server signing keys and certificate: %v\n", err)
				return
			}
		}

		// Generate identity file if --name is provided
		if *serverName != "" {
			if err := keys.GenerateIdentityFile(*keydir, *serverName); err != nil {
				fmt.Printf("error generating identity file: %v\n", err)
				return
			}
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

	// Load federation peers
	peers, err := server.LoadPeers(serverCfg.FederationPeers)
	if err != nil {
		log.Fatalf("Failed to load federation peers: %v", err)
	}

	// Initialize Strike server
	if err := bootstrap.InitStrikeServer(creds, peers); err != nil {
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
