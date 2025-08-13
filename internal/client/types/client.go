package types

import (
	"database/sql"
	// "sync"

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
	Cache Cache
	Shell *ShellState
}

type Cache struct {
	// mu             sync.RWMutex
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

type User struct {
	Id     uuid.UUID
	Name   string
	Enckey []byte
	Sigkey []byte
	KeyEx  int
}

type Message struct {
	Id        uuid.UUID
	FriendId  uuid.UUID
	Direction string
	Content   []byte
	Timestamp int64
}

type FriendRequest struct {
	FriendId  uuid.UUID
	Username  string
	Enckey    []byte
	Sigkey    []byte
	Direction string
}

// Prepared statemnt groups
type ClientDB struct {
	Friends struct {
		SaveUserDetails *sql.Stmt
		GetUserId       *sql.Stmt
		GetUser         *sql.Stmt
		GetFriends      *sql.Stmt
		GetKeyEx        *sql.Stmt
		ConfirmKeyEx    *sql.Stmt
	}

	ID struct {
		GetID  *sql.Stmt
		GetUID *sql.Stmt
		SaveID *sql.Stmt
	}

	Messages struct {
		SaveMessage *sql.Stmt
		GetMessages *sql.Stmt
	}

	FriendRequest struct {
		SaveFriendRequest   *sql.Stmt
		GetFriendRequests   *sql.Stmt
		DeleteFriendRequest *sql.Stmt
	}
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
