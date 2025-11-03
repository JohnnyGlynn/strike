package server

import (
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
