package server

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/JohnnyGlynn/strike/internal/server/types"
	"gopkg.in/yaml.v3"

	pb "github.com/JohnnyGlynn/strike/msgdef/federation"
)

type PeerManager struct {
	peers map[string]*types.PeerRuntime
	mu    sync.RWMutex
}

type FederationOrchestrator struct {
	pb.UnimplementedFederationServer

	strike *StrikeServer
}

func NewFederationOrchestrator(s *StrikeServer) *FederationOrchestrator {
	return &FederationOrchestrator{
		strike: s,
	}
}

func (fo *FederationOrchestrator) Handshake(
	ctx context.Context,
	req *pb.HandshakeReq,
) (*pb.HandshakeAck, error) {

	if req.ServerId == "" {
		return &pb.HandshakeAck{
			Ok:      false,
			Message: "missing server_id",
		}, nil
	}

	return &pb.HandshakeAck{
		Ok:       true,
		ServerId: fo.strike.ID.String(),
		Message:  "handshake accepted",
	}, nil
}

func (fo *FederationOrchestrator) Relay(
	ctx context.Context,
	rp *pb.RelayPayload,
) (*pb.RelayAck, error) {

	if rp == nil || rp.Sender == nil || rp.Recipient == nil {
		return &pb.RelayAck{
			EnvelopeId: rp.GetEnvelopeId(),
			Accepted:   false,
			Info:       "invalid payload",
		}, nil
	}

	if err := fo.strike.EnqueueFederated(ctx, rp); err != nil {
		return &pb.RelayAck{
			EnvelopeId: rp.EnvelopeId,
			Accepted:   false,
			Info:       err.Error(),
		}, nil
	}

	return &pb.RelayAck{
		EnvelopeId: rp.EnvelopeId,
		Accepted:   true,
		Info:       "accepted",
	}, nil
}

func LoadPeers(path string) ([]types.PeerConfig, error) {
	peerConfig, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg types.FederationConfig
	if err := yaml.Unmarshal(peerConfig, &cfg); err != nil {
		return nil, err
	}

	fmt.Printf("cfg: %v\n", cfg)

	fmt.Println("Available peers")
	for _, p := range cfg.Peers {
		fmt.Printf("%s@%s\n", p.Name, p.Address)
	}

	return cfg.Peers, nil
}

