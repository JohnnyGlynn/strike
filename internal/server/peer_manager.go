package server

import (
	"context"
	"crypto/tls"
  "sync"
	"google.golang.org/grpc/credentials"

	"google.golang.org/grpc"

	"github.com/JohnnyGlynn/strike/internal/server/types"
	fedpb "github.com/JohnnyGlynn/strike/msgdef/federation"
)

type PeerManager struct {
	peers map[string]*types.PeerRuntime
	mu    sync.RWMutex
}

func NewPeerManager(cfgs []types.PeerConfig) *PeerManager {
	pm := &PeerManager{
		peers: make(map[string]*types.PeerRuntime),
	}

	for _, c := range cfgs {
		pm.peers[c.ID.String()] = &types.PeerRuntime{
			Cfg: c,
		}
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

	peer.Conn = conn
	peer.Client = client
	peer.Online = true
}

func (pm *PeerManager) Client(peerID string) (fedpb.FederationClient, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	p, ok := pm.peers[peerID]
	if !ok || !p.Online || p.Client == nil {
		return nil, false
	}

	return p.Client, true
}
