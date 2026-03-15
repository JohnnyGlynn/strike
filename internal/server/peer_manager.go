package server

import (
	"context"
	"crypto/tls"
	"google.golang.org/grpc/credentials"
	"sync"

	"google.golang.org/grpc"

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

	conn, err := grpc.NewClient(peer.Cfg.Address, grpc.WithTransportCredentials(creds))
	if err != nil {
		return
	}

	client := fedpb.NewFederationClient(conn)

	_, err = client.Handshake(ctx, &fedpb.HandshakeReq{
		ServerId:   localID,
		ServerName: localName,
	})
	if err != nil {
		conn.Close()
		return
	}

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
