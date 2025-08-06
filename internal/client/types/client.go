package types

import (
	"database/sql"
	"sync"

	"github.com/JohnnyGlynn/strike/internal/config"
	pb "github.com/JohnnyGlynn/strike/msgdef/message"
	"github.com/google/uuid"
)

type ClientIdentity struct {
	Username string
	ID       uuid.UUID
	Keys     map[string][]byte
	Config   *config.ClientConfig
}

type ClientState struct {
	Cache      Cache
	Shell *ShellState
}

type Cache struct {
	mu             sync.RWMutex
	FriendRequests map[uuid.UUID]*pb.FriendRequest
	CurrentChat    ChatSession
}

type ChatSession struct {
	User         User
	SharedSecret []byte
	EncKey       []byte
	HmacKey      []byte
	//TODO:IsGroup
}

type Client struct {
	Identity *ClientIdentity
	State    *ClientState
	PBC      pb.StrikeClient
	DB       *ClientDB
}

// type ClientInfo struct {
// 	Config      *config.ClientConfig
// 	Pbclient    pb.StrikeClient
// 	Keys        map[string][]byte
// 	Username    string
// 	UserID      uuid.UUID
// 	Cache       Cache
// 	Pstatements *ClientDB
// 	//TODO: Cache?
// 	Shell *ShellState
// }

type User struct {
	Id     uuid.UUID
	Name   string
	Enckey []byte
	Sigkey []byte
	KeyEx  int
}

type MessageStruct struct {
	Id        uuid.UUID
	FriendId  uuid.UUID
	Direction string
	Content   []byte
	Timestamp int64
}

// TODO:User?
type FriendRequest struct {
	FriendId  uuid.UUID
	Username  string
	Enckey    []byte
	Sigkey    []byte
	Direction string
}

type ClientDB struct {
	SaveUserDetails     *sql.Stmt
	GetUserId           *sql.Stmt
	GetUser             *sql.Stmt
	GetID               *sql.Stmt
	GetUID              *sql.Stmt
	SaveID              *sql.Stmt
	GetFriends          *sql.Stmt
	GetKeyEx            *sql.Stmt
	ConfirmKeyEx        *sql.Stmt
	SaveMessage         *sql.Stmt
	GetMessages         *sql.Stmt
	SaveFriendRequest   *sql.Stmt
	GetFriendRequests   *sql.Stmt
	DeleteFriendRequest *sql.Stmt
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
	CmdFn func(args []string, client *Client) error
	Scope []ShellMode
}

type ParsedInput struct {
	IsCommand bool
	Command   string
	Args      []string
	Raw       string
}
