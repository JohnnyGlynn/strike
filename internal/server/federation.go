package server

import (
	"os"
	"sync"

	"github.com/JohnnyGlynn/strike/internal/server/types"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

type FederationOrchestrator struct {
	peers    map[string]*types.Peer
	presence map[uuid.UUID]string
	mu       sync.RWMutex
}

// TODO: Load Peers from config
func NewFederationOrchestrator(peers []types.PeerConfig) *FederationOrchestrator {

	fo := &FederationOrchestrator{
		peers: make(map[string]*types.Peer, len(peers)),
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

func LoadPeers(path string) ([]types.PeerConfig, error) {
	peerConfig, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg []types.PeerConfig
	if err := yaml.Unmarshal(peerConfig, &cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}
