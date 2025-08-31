package types

import (
	"sync"
	"time"

	pb "github.com/JohnnyGlynn/strike/msgdef/message"
	"github.com/google/uuid"
	"google.golang.org/grpc"
)

type PendingMsg struct {
	MessageID uuid.UUID
	From      uuid.UUID
	To        uuid.UUID
	Payload   *pb.StreamPayload
	Created   time.Time
	Attempts  int
}

type PeerConfig struct {
  ID string
  Name string
  Address string
  PubKey []byte
  // TLS bool
  //Cert
}

type Peer struct {
  Config PeerConfig
  Mu *sync.Mutex
  Conn *grpc.ClientConn

  LastComms time.Time
}
