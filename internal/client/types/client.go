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
	//TODO: Cache?
	Shell *ShellState
}

type Cache struct {
	FriendRequests map[uuid.UUID]*pb.FriendRequest
	Chats          map[uuid.UUID]*pb.Chat
	CurrentChat    ChatDetails
}

type User struct {
	Id     uuid.UUID
	Name   string
	Enckey []byte
	Sigkey []byte
	KeyEx  int
}

type ChatDetails struct {
	User         User
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
	GetUser         *sql.Stmt
	GetID           *sql.Stmt
	SaveID          *sql.Stmt
	GetFriends      *sql.Stmt
	GetKeyEx        *sql.Stmt
	ConfirmKeyEx    *sql.Stmt
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
	CmdFn func(args []string, client *ClientInfo) error
	Scope []ShellMode
}

type ParsedInput struct {
	IsCommand bool
	Command   string
	Args      []string
	Raw       string
}
