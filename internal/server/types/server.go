package types

import (
	"crypto/ed25519"
	"sync"
	"time"

	"github.com/JohnnyGlynn/strike/msgdef/common"
	fedpb "github.com/JohnnyGlynn/strike/msgdef/federation"
	"github.com/google/uuid"
	"google.golang.org/grpc"
)

type PendingMsg struct {
	MessageID   uuid.UUID
	From        uuid.UUID
	To          uuid.UUID
	Payload     *common.EncryptedEnvelope
	Origin      string
	Destination string
	Created     time.Time
	Attempts    int
}

type PeerConfig struct {
	ID      uuid.UUID         `yaml:"id"`
	Name    string            `yaml:"name"`
	Address string            `yaml:"addr"`
	PubKey  ed25519.PublicKey `yaml:"pubkey"`
}

type PeerRuntime struct {
	Cfg        PeerConfig
	Mu         sync.RWMutex
	Conn       *grpc.ClientConn
	Client     fedpb.FederationClient
	Handshaken bool
	LastSeen   time.Time
	Online     bool
}

type Peer struct {
	Config PeerConfig
	Mu     *sync.Mutex
	Conn   *grpc.ClientConn

	LastComms time.Time
}

type FederationConfig struct {
	Peers []PeerConfig `yaml:"peers"`
}
