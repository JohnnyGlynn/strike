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
	"github.com/JohnnyGlynn/strike/internal/server/types"
	fedpb "github.com/JohnnyGlynn/strike/msgdef/federation"
	pb "github.com/JohnnyGlynn/strike/msgdef/message"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type Bootstrap struct {
	Cfg        config.ServerConfig
	DB         *pgxpool.Pool
	Statements *ServerDB

	Strike       *StrikeServer
	Orchestrator *FederationOrchestrator

	grpcStrike *grpc.Server
	grpcFed    *grpc.Server

	fedTLS *tls.Config
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

func (b *Bootstrap) InitStrikeServer(creds credentials.TransportCredentials, peers []types.PeerConfig) error {
	key, err := keys.GetKeyFromPath(b.Cfg.SigningPublicKeyPath)
	if err != nil {
		return err
	}

	id := DeriveServerID(key)

	b.Strike = &StrikeServer{
		Name:           "strike-server",
		ID:             uuid.MustParse(id),
		DBpool:         b.DB,
		PStatements:    b.Statements,
		PeerMgr:        NewPeerManager(peers),
		Pending:        make(map[uuid.UUID]*types.PendingMsg),
		RemotePresence: make(map[uuid.UUID]string),
	}
	b.grpcStrike = grpc.NewServer(
		grpc.Creds(creds),
	)

	pb.RegisterStrikeServer(b.grpcStrike, b.Strike)
	return nil
}

func (b *Bootstrap) InitFederation() error {
	var err error

	b.fedTLS, err = LoadFederationTLSConfig(
		b.Cfg.CertificatePath,
		b.Cfg.SigningPrivateKeyPath,
		b.Cfg.FederationCAPath,
	)
	if err != nil {
		return err
	}

	b.Orchestrator = NewFederationOrchestrator(b.Strike)

	b.grpcFed = grpc.NewServer(
		grpc.Creds(credentials.NewTLS(b.fedTLS)),
	)

	fedpb.RegisterFederationServer(b.grpcFed, b.Orchestrator)
	return nil
}

func LoadFederationTLSConfig(certFile, keyFile, caFile string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}

	caPEM, err := os.ReadFile(caFile)
	if err != nil {
		return nil, err
	}

	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("invalid CA pem")
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientCAs:    caPool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		MinVersion:   tls.VersionTLS13,
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
	if b.grpcStrike == nil {
		return fmt.Errorf("grpcStrike not initialized")
	}
	if b.grpcFed == nil {
		return fmt.Errorf("grpcFed not initialized")
	}
	if b.Strike == nil {
		return fmt.Errorf("strike server not initialized")
	}
	if b.Strike.PeerMgr == nil {
		return fmt.Errorf("peer manager not initialized")
	}

	go func() {
		lis, err := net.Listen("tcp", "0.0.0.0:8080")
		if err != nil {
			fmt.Printf("strike listen failed: %v\n", err)
			return
		}
		defer lis.Close()

		fmt.Println("strike server listening on :8080")

		if err := b.grpcStrike.Serve(lis); err != nil {
			fmt.Printf("strike serve error: %v\n", err)
		}
	}()

	go func() {
		lis, err := net.Listen("tcp", "0.0.0.0:9090")
		if err != nil {
			fmt.Printf("federation listen failed: %v\n", err)
			return
		}
		defer lis.Close()

		fmt.Println("federation server listening on :9090")

		if err := b.grpcFed.Serve(lis); err != nil {
			fmt.Printf("federation serve error: %v\n", err)
		}
	}()

	go func() {
		// federation TLS (mTLS)
		tlsConf, err := b.InitFederationTLS()
		if err != nil {
			fmt.Printf("federation TLS init failed: %v\n", err)
			return
		}

		localID := b.Strike.ID.String()
		localName := b.Strike.Name

		b.Strike.PeerMgr.ConnectAll(ctx, tlsConf, localID, localName)

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

	if b.DB != nil {
		b.DB.Close()
	}

	fmt.Println("Shutdown complete")
}
