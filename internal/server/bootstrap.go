package server

import (
	"context"
	"fmt"
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
		fmt.Printf("Config parsing failed: %v", err)
		return err
	}

	b.DB, err = dbWithRetry(ctx, pgConfig)
	if err != nil {
		return err
	}

	fmt.Println("DB connection established...")
	defer b.DB.Close()

	b.Statements, err = PrepareStatements(ctx, b.DB)
	if err != nil {
		fmt.Printf("Failed to prepare statements: %v", err)
		return err
	}

	fmt.Println("Prepared statements")
	return nil

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
