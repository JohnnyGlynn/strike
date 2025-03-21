package db

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ClientDB struct {
	GetUserId       string
	SaveUserDetails string
	SaltMine        string
	CreateChat      string
	UpdateChatState string
	SaveMessage     string
	GetChat         string
	GetChats        string
	GetMessages     string
}

func PreparedClientStatements(ctx context.Context, dbpool *pgxpool.Pool) (*ClientDB, error) {
	poolConnection, err := dbpool.Acquire(ctx)
	if err != nil {
		log.Fatalf("failed to acquire connection from pool: %v", err)
		return nil, err
	}
	defer poolConnection.Release()

	statements := &ClientDB{
		GetUserId:       "getUserId",
		SaveUserDetails: "saveUserDetails",
		CreateChat:      "createChat",
		GetChat:         "getChat",
		GetChats:        "getChats",
		UpdateChatState: "UpdateChatState",
		SaveMessage:     "saveMessage",
		GetMessages:     "getMessages",
	}

	// Insert into address book
	if _, err := poolConnection.Conn().Prepare(ctx, statements.SaveUserDetails, "INSERT INTO addressbook (user_id, username, encryption_public_key, signing_public_key) VALUES ($1, $2, $3, $4)"); err != nil {
		return nil, err
	}

	// Get User Id
	if _, err := poolConnection.Conn().Prepare(ctx, statements.GetUserId, "SELECT user_id FROM addressbook WHERE username = $1;"); err != nil {
		return nil, err
	}

	// Insert Chat - Create a chat, containing secret key after successful key exchange.
	if _, err := poolConnection.Conn().Prepare(ctx, statements.CreateChat, "INSERT INTO chats (chat_id, chat_name, initiator, participants, state, shared_secret) VALUES ($1, $2, $3, $4, $5, $6)"); err != nil {
		return nil, err
	}

	// get chat
	if _, err := poolConnection.Conn().Prepare(ctx, statements.GetChat, "SELECT * FROM chats WHERE chat_id = $1"); err != nil {
		return nil, err
	}

	// get chats
	if _, err := poolConnection.Conn().Prepare(ctx, statements.GetChats, "SELECT * FROM chats"); err != nil {
		return nil, err
	}

	// Updated Chat state
	if _, err := poolConnection.Conn().Prepare(ctx, statements.UpdateChatState, "UPDATE chats SET state = $1 WHERE chat_id = $2"); err != nil {
		return nil, err
	}

	// Insert Message - Insert message bound by chat
	if _, err := poolConnection.Conn().Prepare(ctx, statements.SaveMessage, "INSERT INTO messages (id, chat_id, sender, content) VALUES ($1, $2, $3, $4)"); err != nil {
		return nil, err
	}

	// get all messages
	if _, err := poolConnection.Conn().Prepare(ctx, statements.GetMessages, "SELECT * FROM messages WHERE chat_id = $1 ORDER BY sent_at ASC, id ASC;"); err != nil {
		return nil, err
	}

	return statements, nil
}
