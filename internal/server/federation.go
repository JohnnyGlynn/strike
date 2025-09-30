package server

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/JohnnyGlynn/strike/internal/server/types"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"gopkg.in/yaml.v3"

	pb "github.com/JohnnyGlynn/strike/msgdef/federation"
)

type FederationOrchestrator struct {
  pb.UnimplementedFederationServer
	peers    map[string]*types.Peer
	presence map[uuid.UUID]string
	clients  map[string]pb.FederationClient
	mu       sync.RWMutex

  strike *StrikeServer //backref
}

// TODO: Load Peers from config
func NewFederationOrchestrator(peers []types.PeerConfig) *FederationOrchestrator {

	fo := &FederationOrchestrator{
		peers:   make(map[string]*types.Peer, len(peers)),
		clients: make(map[string]pb.FederationClient),
	}

	for _, cfg := range peers {
		p := &types.Peer{Config: cfg}
		fo.peers[cfg.ID] = p
	}

	return fo
}

func (fo *FederationOrchestrator) UpdatePresence(user uuid.UUID, origin string) {
	fo.mu.Lock()
	defer fo.mu.Unlock()
	fo.presence[user] = origin
}

func (fo *FederationOrchestrator) Lookup(user uuid.UUID) (string, bool) {
	fo.mu.RLock()
	defer fo.mu.RUnlock()
	origin, ok := fo.presence[user]
	return origin, ok
}

func (fo *FederationOrchestrator) PeerClient(peerId string) (pb.FederationClient, error) {
	fo.mu.RLock()
	client, ok := fo.clients[peerId]
	fo.mu.RUnlock()
	if ok {
		return client, nil
	}

	fo.mu.Lock()
	defer fo.mu.Unlock()

	peer, ok := fo.peers[peerId]
	if !ok {
		return nil, fmt.Errorf("peer not found: %s", peerId)
	}

	conn, err := grpc.NewClient(peer.Config.Address)
	if err != nil {
		return nil, err
	}

	client = pb.NewFederationClient(conn)
	fo.clients[peerId] = client

	return client, nil

}

func (fo *FederationOrchestrator) Ping(ctx context.Context, peerID string) (*pb.PingAck, error) {

	grpcClient, err := fo.PeerClient(peerID)
	if err != nil {
		return &pb.PingAck{}, err
	}

	ack, err := grpcClient.Ping(ctx, &pb.PingReq{
		OriginId: "TODO-load-server-id-from-config",
	})
	if err != nil {
		return &pb.PingAck{}, err
	}

	fmt.Printf("Peer %s Acknowledged: %v\n", peerID, ack.Ok)

	return ack, nil
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

	return cfg.Peers, nil
}

func (fo *FederationOrchestrator) RoutePayload(ctx context.Context, fp *pb.FedPayload) (*pb.FedAck, error) {
  if fp == nil || fp.Sender == nil || fp.Recipient == nil {
    return &pb.FedAck{Accepted: false}, fmt.Errorf("invalid federated payload")
  }

  from, err := uuid.Parse(fp.Sender.UInfo.UserId)
  if err != nil {
    return &pb.FedAck{Accepted: false}, fmt.Errorf("bad sender id")
  }

  to, err := uuid.Parse(fp.Recipient.UInfo.UserId)
  if err != nil {
    return &pb.FedAck{Accepted: false}, fmt.Errorf("bad reciever id")
  }

  msgID := uuid.New()//Add to message/pending?
  pmsg := &types.PendingMsg{
    From: from,
    To: to,
    Payload: fp.Payload,
    Attempts: 0,
    Destination: "local",
  }


  s := fo.strike
  s.mu.Lock()
  s.Pending[msgID] = pmsg
  s.mu.Unlock()

  go s.attemptDelivery(ctx, msgID)

	return &pb.FedAck{Accepted: true}, nil
}
