package server

import (
  "sync"
  "github.com/JohnnyGlynn/strike/internal/server/types"
)

type FederationOrchestrator struct {
  peers map[string]*types.Peer
  mu sync.RWMutex
}

//TODO: Load Peers from config
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
