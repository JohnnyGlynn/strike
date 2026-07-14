package server

import (
	"context"
	"crypto/tls"
	"log"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/JohnnyGlynn/strike/internal/server/types"
	fedpb "github.com/JohnnyGlynn/strike/msgdef/federation"
)

type PeerManager struct {
	mu      sync.RWMutex
	peers   map[string]*types.PeerRuntime
	conns   map[string]*grpc.ClientConn
	clients map[string]fedpb.FederationClient
}

func NewPeerManager(peers []types.PeerConfig) *PeerManager {
	pm := &PeerManager{
		peers:   make(map[string]*types.PeerRuntime, len(peers)),
		conns:   make(map[string]*grpc.ClientConn),
		clients: make(map[string]fedpb.FederationClient),
	}
	for _, p := range peers {
		pm.peers[p.ID.String()] = &types.PeerRuntime{Cfg: p}
	}
	return pm
}

func (pm *PeerManager) ConnectAll(
	ctx context.Context,
	tlsConf *tls.Config,
	localID string,
	localName string,
) {

	pm.mu.RLock()
	defer pm.mu.RUnlock()

	for _, peer := range pm.peers {
		if peer.Cfg.Name == localName {
			continue
		}
		go pm.connectPeer(ctx, peer, tlsConf, localID, localName)
	}
}

func (pm *PeerManager) connectPeer(
	ctx context.Context,
	peer *types.PeerRuntime,
	tlsConf *tls.Config,
	localID string,
	localName string,
) {

	creds := credentials.NewTLS(tlsConf)

	log.Printf("federation: connecting to peer %s@%s", peer.Cfg.Name, peer.Cfg.Address)

	conn, err := grpc.NewClient(peer.Cfg.Address, grpc.WithTransportCredentials(creds))
	if err != nil {
		log.Printf("federation: dial failed for %s: %v", peer.Cfg.Name, err)
		return
	}

	client := fedpb.NewFederationClient(conn)

	backoff := 2 * time.Second
	for attempt := 1; attempt <= 10; attempt++ {
		_, err = client.Handshake(ctx, &fedpb.HandshakeReq{
			ServerId:   localID,
			ServerName: localName,
		})
		if err == nil {
			break
		}

		log.Printf("federation: handshake attempt %d failed for %s: %v", attempt, peer.Cfg.Name, err)

		select {
		case <-ctx.Done():
			conn.Close()
			return
		case <-time.After(backoff):
		}

		if backoff < 30*time.Second {
			backoff *= 2
		}
	}

	if err != nil {
		log.Printf("federation: giving up on peer %s after retries", peer.Cfg.Name)
		conn.Close()
		return
	}

	log.Printf("federation: connected to peer %s", peer.Cfg.Name)

	pm.mu.Lock()
	pm.conns[peer.Cfg.ID.String()] = conn
	pm.clients[peer.Cfg.ID.String()] = client
	pm.mu.Unlock()

	peer.Mu.Lock()
	peer.Conn = conn
	peer.Client = client
	peer.Online = true
	peer.Handshaken = true
	peer.Mu.Unlock()
}

func (pm *PeerManager) Client(peerID string) (fedpb.FederationClient, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	c, ok := pm.clients[peerID]
	return c, ok
}

// ClientByName looks up a federation peer by its server name (domain).
func (pm *PeerManager) ClientByName(name string) (fedpb.FederationClient, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	for _, peer := range pm.peers {
		if peer.Cfg.Name == name {
			c, ok := pm.clients[peer.Cfg.ID.String()]
			return c, ok
		}
	}
	return nil, false
}
