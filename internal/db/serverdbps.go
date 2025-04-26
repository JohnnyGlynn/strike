package db

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ServerDB struct {
	CreateUser       string
	LoginUser        string
	GetPublicKeys    string
	GetUserId        string
  GetUsername        string
	CreatePublicKeys string
	SaltMine         string
	CreateChat       string
	SaveMessage      string
	GetChat          string
	GetMessages      string
}

func PrepareStatements(ctx context.Context, dbpool *pgxpool.Pool) (*ServerDB, error) {
	poolConnection, err := dbpool.Acquire(ctx)
	if err != nil {
		log.Fatalf("failed to acquire connection from pool: %v", err)
		return nil, err
	}
	defer poolConnection.Release()

	statements := &ServerDB{
		CreateUser:       "createUser",
		LoginUser:        "loginUser",
		GetPublicKeys:    "getublicKeys",
		GetUserId:        "getUserId",
		GetUsername:      "getUsername",
		CreatePublicKeys: "createPublicKeys",
		SaltMine:         "saltMine",
		CreateChat:       "createChat",
	}

	// LoginUser
	if _, err := poolConnection.Conn().Prepare(ctx, statements.LoginUser, "SELECT password_hash FROM users WHERE user_id = $1"); err != nil {
		return nil, err
	}

	// salt retrieval
	if _, err := poolConnection.Conn().Prepare(ctx, statements.SaltMine, "SELECT salt FROM users WHERE user_id = $1"); err != nil {
		return nil, err
	}

	//
	// Get keys from key table
  //TODO: Fix DB tables 
	if _, err := poolConnection.Conn().Prepare(ctx, statements.GetPublicKeys, "SELECT encryption_public_key, signing_public_key FROM user_keys  WHERE user_id = $1;"); err != nil {
		return nil, err
	}

	// Get User Id
	if _, err := poolConnection.Conn().Prepare(ctx, statements.GetUserId, "SELECT user_id FROM users WHERE username = $1;"); err != nil {
		return nil, err
	}

	// Get User Name TODO:Stopgap?
	if _, err := poolConnection.Conn().Prepare(ctx, statements.GetUsername, "SELECT username FROM users WHERE user_id = $1;"); err != nil {
		return nil, err
	}

	// Insert Users keys
	if _, err := poolConnection.Conn().Prepare(ctx, statements.CreatePublicKeys, "INSERT INTO user_keys (user_id, encryption_public_key, signing_public_key) VALUES ($1, $2, $3)"); err != nil {
		return nil, err
	}

	// Insert Users with password
	if _, err := poolConnection.Conn().Prepare(ctx, statements.CreateUser, "INSERT INTO users (user_id, username, password_hash, salt) VALUES ($1, $2, $3, $4)"); err != nil {
		return nil, err
	}

	// Insert Chat
	if _, err := poolConnection.Conn().Prepare(ctx, statements.CreateChat, "INSERT INTO chats (chat_id, chat_name, initiator, participants, state) VALUES ($1, $2, $3, $4, $5)"); err != nil {
		return nil, err
	}

	// Insert Message
	if _, err := poolConnection.Conn().Prepare(ctx, statements.SaveMessage, "INSERT INTO messages (id, chat_id, sender, content) VALUES ($1, $2, $3, $4)"); err != nil {
		return nil, err
	}

	// get chat - TODO: Mechanism for caching these required?
	if _, err := poolConnection.Conn().Prepare(ctx, statements.GetChat, "SELECT * FROM chats WHERE chat_id = $1"); err != nil {
		return nil, err
	}

	// get all messages
	if _, err := poolConnection.Conn().Prepare(ctx, statements.GetMessages, "SELECT * FROM messages WHERE chat_id = $1"); err != nil {
		return nil, err
	}

	return statements, nil
}
