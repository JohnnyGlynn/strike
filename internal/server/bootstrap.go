package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/JohnnyGlynn/strike/internal/config"
	"github.com/JohnnyGlynn/strike/internal/keys"
	fedpb "github.com/JohnnyGlynn/strike/msgdef/federation"
	pb "github.com/JohnnyGlynn/strike/msgdef/message"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type Bootstrap struct {
	Cfg          config.ServerConfig
	DB           *pgxpool.Pool
	Statements   *ServerDB
	Strike       *StrikeServer
	Orchestrator *FederationOrchestrator
	grpcStrike   *grpc.Server
	grpcFed      *grpc.Server
}

func InitBootstrap(cfg config.ServerConfig) *Bootstrap {
	return &Bootstrap{Cfg: cfg}
}

func (b *Bootstrap) InitDb(ctx context.Context) error {
	pgConfig, err := pgxpool.ParseConfig(b.Cfg.DBConnectionString)
	if err != nil {
		return err
	}

	b.DB, err = dbWithRetry(ctx, pgConfig)
	if err != nil {
		return err
	}

	b.Statements, err = PrepareStatements(ctx, b.DB)
	return err
}

func dbWithRetry(ctx context.Context, pgConfig *pgxpool.Config) (*pgxpool.Pool, error) {
	var pool *pgxpool.Pool
	var err error

	backoff := time.Second

	for i := 0; i < 30; i++ {
		pool, err = pgxpool.NewWithConfig(ctx, pgConfig)
		if err == nil {
			pingErr := pool.Ping(ctx)
			if pingErr == nil {
				return pool, nil
			}
			err = pingErr
		}

		fmt.Printf("DB not ready (%v). Retrying: %s...", err, backoff)

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}

		if backoff < 10*time.Second {
			backoff *= 2
		}
	}

	return nil, fmt.Errorf("db unavailable: %w", err)
}

func (b *Bootstrap) InitServer(creds credentials.TransportCredentials) error {
	key, err := keys.GetKeyFromPath(b.Cfg.SigningPublicKeyPath)
	if err != nil {
		return err
	}

	id := DeriveServerID(key)

	b.Strike = &StrikeServer{
		Name: "strike-server",
		//TODO: Persistent identity
		ID:          uuid.MustParse(id),
		DBpool:      b.DB,
		PStatements: b.Statements,
		// Federation:  orchestrator,
	}

	opts := []grpc.ServerOption{
		grpc.Creds(creds),
	}

	srvr := grpc.NewServer(opts...)
	pb.RegisterStrikeServer(srvr, b.Strike)

	return nil

}

func (b *Bootstrap) InitFederation(creds credentials.TransportCredentials) error {
	peers, err := LoadPeers(b.Cfg.FederationPeers)
	if err != nil {
		fmt.Printf("error loading peers: %v", err)
		return err
	}

	b.Orchestrator = NewFederationOrchestrator(b.Strike, peers)

	fedSrvr := grpc.NewServer()
	fedpb.RegisterFederationServer(fedSrvr, b.Orchestrator)

	return nil

}

func LoadFederationTLSConfig(
	certFile string,
	keyFile string,
	caFile string,
) (*tls.Config, error) {

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("load cert: %w", err)
	}

	caPEM, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("read ca: %w", err)
	}

	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("invalid CA pem")
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientCAs:    caPool,

		ClientAuth: tls.RequireAndVerifyClientCert,

		MinVersion: tls.VersionTLS13,
	}, nil
}

func (b *Bootstrap) InitFederationTLS() (*tls.Config, error) {
	tlsConf, err := LoadFederationTLSConfig(
		b.Cfg.CertificatePath,       // server cert
		b.Cfg.SigningPrivateKeyPath, // server key
		b.Cfg.FederationCAPath,      // CA that signs peer certs
	)
	if err != nil {
		return nil, err
	}

	fmt.Println("Loaded federation mTLS config")
	return tlsConf, nil
}

func (b *Bootstrap) Start(ctx context.Context) error {

	go func() {
		lis, err := net.Listen("tcp", "0.0.0.0:8080")
		if err != nil {
			fmt.Printf("failed to listen: %v", err)
			return
		}

		fmt.Println("strike server: listening on 0.0.0.0:8080")

		err = b.grpcStrike.Serve(lis)
		if err != nil {
			fmt.Printf("Error listening: %v\n", err)
		}
	}()

	go func() {
		lisFed, err := net.Listen("tcp", "0.0.0.0:9090")
		if err != nil {
			fmt.Println("failed to create federation listener")
			return
		}

		fmt.Println("federation server: listening on 0.0.0.0:9090")

		err = b.grpcFed.Serve(lisFed)
		if err != nil {
			fmt.Println("failed to start federation server")
			return
		}
	}()

	go func() {
		err := b.Orchestrator.ConnectPeers(ctx)
		if err != nil {
			fmt.Printf("failed peer connection: %v", err)
		}
	}()

	return nil

}

func (b *Bootstrap) Stop(ctx context.Context) {

	fmt.Println("Strike shutting down")

	if b.grpcStrike != nil {
		fmt.Println("shutdown strike server")
	}

	if b.grpcFed != nil {
		fmt.Println("shutdown strike federation server")
	}

	if b.Orchestrator != nil {
		_ = b.Orchestrator.Close()
	}

	if b.DB != nil {
		b.DB.Close()
	}

	fmt.Println("Shutdown complete")
}
