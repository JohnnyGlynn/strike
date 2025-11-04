package server

import (
	"context"
	"fmt"
	"time"

	"github.com/JohnnyGlynn/strike/internal/config"

	"github.com/jackc/pgx/v5/pgxpool"
	grpc "google.golang.org/grpc"
)

type Bootstrap struct {
	Cfg          config.ServerConfig
	DB           *pgxpool.Pool
	Statements   *ServerDB
	Strike       *StrikeServer
	Orchestrator FederationOrchestrator
	grpcStrike   *grpc.Server
	grpcFed      *grpc.Server
}

func InitBootstrap(cfg config.ServerConfig) *Bootstrap {
	return &Bootstrap{Cfg: cfg}
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

// func (b *Bootstrap)
