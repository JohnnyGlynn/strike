package server

import (
	"context"
	"fmt"
	"maps"
	"os"
	"sync"

	"github.com/JohnnyGlynn/strike/internal/server/types"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"gopkg.in/yaml.v3"

	pb "github.com/JohnnyGlynn/strike/msgdef/federation"
)

type FederationOrchestrator struct {
	pb.UnimplementedFederationServer
	peers       map[string]*types.Peer
	presence    map[uuid.UUID]string
	clients     map[string]pb.FederationClient
	connections map[string]*grpc.ClientConn

	mu     sync.RWMutex
	strike *StrikeServer //backref
}

// TODO: Load Peers from config
func NewFederationOrchestrator(s *StrikeServer, peers []types.PeerConfig) *FederationOrchestrator {

	fo := &FederationOrchestrator{
		peers:       make(map[string]*types.Peer, len(peers)),
		presence:    make(map[uuid.UUID]string),
		clients:     make(map[string]pb.FederationClient),
		connections: make(map[string]*grpc.ClientConn),
		strike:      s,
	}

	for _, cfg := range peers {
		fo.peers[cfg.ID] = &types.Peer{Config: cfg}
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

func (fo *FederationOrchestrator) Close() error {
	fo.mu.Lock()
	defer fo.mu.Unlock()

	for id, conn := range fo.connections {
		if conn != nil {
			err := conn.Close()
			if err != nil {
				fmt.Printf("failed to closed connection for %s: %v", id, err)
				return err
			}
			fmt.Printf("Connection with %s closed.", id)
		}
	}

	return nil
}

func (fo *FederationOrchestrator) PeerClient(peerId string) (pb.FederationClient, bool) {
	fo.mu.RLock()
	defer fo.mu.RUnlock()

	client, ok := fo.clients[peerId]
	return client, ok

}

func (fo *FederationOrchestrator) ConnectPeers(ctx context.Context) error {
	fo.mu.RLock()
	copyPeers := make(map[string]*types.Peer, len(fo.peers))
	maps.Copy(fo.peers, copyPeers)
	fo.mu.RUnlock()

	var wg sync.WaitGroup
	errCh := make(chan error, len(copyPeers))

	for id, peer := range copyPeers {

		if peer.Config.ID == fo.strike.ID.String() {
			fmt.Println("Federation: skipping self")
			continue
		}

		wg.Add(1)

		go func(id string, peer *types.Peer) {
			defer wg.Done()

			//TODO: CREDS
			conn, err := grpc.NewClient(peer.Config.Address, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				errCh <- fmt.Errorf("failed to connect peer %s, err:%v\n", id, err)
				return
			}

			client := pb.NewFederationClient(conn)

			fo.mu.Lock()
			fo.clients[id] = client
			fo.connections[id] = conn
			fo.mu.Unlock()

			fmt.Printf("Connecting to peer: %s:%s\n", id, peer.Config.Address)

			pong, err := client.Ping(ctx, &pb.PingReq{OriginId: fo.strike.ID.String(), DestinationId: peer.Config.ID, DestinationAddr: peer.Config.Address})
			if err != nil {
				errCh <- fmt.Errorf("failed to ping peer %s, err:%v\n", id, err)
				return
			}
			fmt.Printf("Peer %s: ok", pong.AckBy)

		}(id, peer)

	}

	wg.Wait()
	close(errCh)

	var errs []error
	for e := range errCh {
		errs = append(errs, e)
	}
	if len(errs) > 0 {
		return fmt.Errorf("%d peers failed to connect: %v", len(errs), errs)

	}

	return nil
}

func (fo *FederationOrchestrator) Ping(ctx context.Context, pr *pb.PingReq) (*pb.PingAck, error) {

	grpcClient, ok := fo.PeerClient(pr.DestinationId)
	if !ok {
		return &pb.PingAck{}, fmt.Errorf("no peer")
	}

	//TODO:DRY?
	ack, err := grpcClient.Ping(ctx, &pb.PingReq{
		OriginId:        "TODO-load-server-id-from-config",
		DestinationId:   pr.DestinationId,
		DestinationAddr: pr.DestinationAddr,
	})
	if err != nil {
		return &pb.PingAck{}, err
	}

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

	fmt.Printf("cfg: %v\n", cfg)

	fmt.Println("Available peers")
	for _, p := range cfg.Peers {
		fmt.Printf("%s@%s\n", p.Name, p.Address)
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

	msgID := uuid.New() //Add to message/pending?
	pmsg := &types.PendingMsg{
		From:        from,
		To:          to,
		Payload:     fp.Payload,
		Attempts:    0,
		Destination: "local",
	}

	s := fo.strike
	s.mu.Lock()
	s.Pending[msgID] = pmsg
	s.mu.Unlock()

	go s.attemptDelivery(ctx, msgID)

	return &pb.FedAck{Accepted: true}, nil
}
