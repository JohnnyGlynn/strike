package types

import (
	"database/sql"

	"github.com/JohnnyGlynn/strike/internal/config"
	pb "github.com/JohnnyGlynn/strike/msgdef/message"
	"github.com/google/uuid"
)

type ClientInfo struct {
	Config      *config.ClientConfig
	Pbclient    pb.StrikeClient
	Keys        map[string][]byte
	Username    string
	UserID      uuid.UUID
	Cache       ClientCache
	Pstatements *ClientDB
}

type ClientCache struct {
	Invites    map[uuid.UUID]*pb.BeginChatRequest
  FriendRequests map[uuid.UUID]*pb.FriendRequest
	Chats      map[uuid.UUID]*pb.Chat
	ActiveChat ChatDetails
}

// In memory persistence for shared secret and derived keys
type ChatDetails struct {
	Chat         *pb.Chat
	SharedSecret []byte
	EncKey       []byte
	HmacKey      []byte
}

// TODO: CLEAN THIS UP
type MessageStruct struct {
	Id        uuid.UUID
	ChatId    uuid.UUID
	Sender    uuid.UUID
	Receiver  uuid.UUID
	Direction string
	Content   []byte
	Timestamp int64
}

type ClientDB struct {
	SaveUserDetails *sql.Stmt
	GetUserId       *sql.Stmt
	GetUsername     *sql.Stmt
	CreateChat      *sql.Stmt
	GetChat         *sql.Stmt
	GetChats        *sql.Stmt
	UpdateChatState *sql.Stmt
	SaveMessage     *sql.Stmt
	GetMessages     *sql.Stmt
}
