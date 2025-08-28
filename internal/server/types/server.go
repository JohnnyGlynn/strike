package types

import (
	pb "github.com/JohnnyGlynn/strike/msgdef/message"
	"github.com/google/uuid"
	"time"
)

type PendingMsg struct {
	MessageID uuid.UUID
	From      uuid.UUID
	To        uuid.UUID
	Payload   *pb.StreamPayload
	Created   time.Time
	Attempts  int
}
