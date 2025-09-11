package types

import (
	"sync"
	"time"

	pb "github.com/JohnnyGlynn/strike/msgdef/message"
	"github.com/google/uuid"
	"google.golang.org/grpc"
)

type PendingMsg struct {
	MessageID   uuid.UUID
	From        uuid.UUID
	To          uuid.UUID
	Payload     *pb.StreamPayload
	Origin      string
	Destination string
	Created     time.Time
	Attempts    int
}

type PeerConfig struct {
	ID      string `yaml:"id"`
	Name    string `yaml:"name"`
	Address string `yaml:"addr"`
	PubKey  []byte `yaml:"pubkey"`
	// TLS bool
	//Cert
}

type FederationConfig struct {
	Peers []PeerConfig `yaml:"peers"`
}

type Peer struct {
	Config PeerConfig
	Mu     *sync.Mutex
	Conn   *grpc.ClientConn

	LastComms time.Time
}
