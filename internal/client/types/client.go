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
	Cache       Cache
	Pstatements *ClientDB
}

type Cache struct {
	FriendRequests map[uuid.UUID]*pb.FriendRequest
	Chats          map[uuid.UUID]*pb.Chat
	CurrentChat    ChatDetails
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
	GetFriends      *sql.Stmt
	CreateChat      *sql.Stmt
	GetChat         *sql.Stmt
	GetChats        *sql.Stmt
	UpdateChatState *sql.Stmt
	SaveMessage     *sql.Stmt
	GetMessages     *sql.Stmt
}

type ShellMode int

const (
	ModeDefault ShellMode = iota
	ModeChat
)

type ShellState struct {
	Mode ShellMode
}

type Command struct {
	Name  string
	Desc  string
	CmdFn func(args []string, state *ShellState, client *ClientInfo)
	Scope []ShellMode
}

type ParsedInput struct {
	IsCommand bool
	Command   string
	Args      []string
	Raw       string
}
