package client

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/JohnnyGlynn/strike/internal/types"
)

func PrepareStatements(ctx context.Context, db *sql.DB) (*types.ClientDB, error) {
	var err error
	statements := &types.ClientDB{}

	// Insert into address book
	if statements.SaveUserDetails, err = db.PrepareContext(ctx, `INSERT INTO addressbook (user_id, username, enc_pkey, sig_pkey) VALUES (?, ?, ?, ?) ON CONFLICT(user_id) DO UPDATE SET username=excluded.username, enc_pkey=excluded.enc_pkey, sig_pkey=excluded.sig_pkey;`); err != nil {
		return nil, err
	}

	// Get User Id
	if statements.GetUserId, err = db.PrepareContext(ctx, `SELECT user_id FROM addressbook WHERE username = ?;`); err != nil {
		return nil, err
	}

	if statements.GetUsername, err = db.PrepareContext(ctx, `SELECT username FROM addressbook WHERE user_id = ?;`); err != nil {
		return nil, err
	}

	if statements.GetFriends, err = db.PrepareContext(ctx, `SELECT * FROM addressbook`); err != nil {
		return nil, err
	}

	// Insert Chat
	if statements.CreateChat, err = db.PrepareContext(ctx, `INSERT INTO chats (chat_id, chat_name, initiator, participants, state) VALUES (?, ?, ?, ?, ?)`); err != nil {
		return nil, err
	}

	// get chat
	if statements.GetChat, err = db.PrepareContext(ctx, `SELECT * FROM chats WHERE chat_id = ?`); err != nil {
		return nil, err
	}

	// get chats
	if statements.GetChats, err = db.PrepareContext(ctx, `SELECT * FROM chats`); err != nil {
		return nil, err
	}

	// Updated Chat state
	if statements.UpdateChatState, err = db.PrepareContext(ctx, `UPDATE chats SET state = ? WHERE chat_id = ?`); err != nil {
		return nil, err
	}

	// Insert Message - Insert message bound by chat
	if statements.SaveMessage, err = db.PrepareContext(ctx, `INSERT INTO messages (id, chat_id, sender, receiver, direction, content, timestamp) VALUES (?, ?, ?, ?, ?, ?, ?)`); err != nil {
		return nil, err
	}

	// get all messages
	if statements.GetMessages, err = db.PrepareContext(ctx, `SELECT * FROM messages WHERE chat_id = ? ORDER BY timestamp ASC, id ASC;`); err != nil {
		return nil, err
	}

	return statements, nil
}

func CloseStatements(c *types.ClientDB) error {
	var errs []error

	closeStmt := func(stmt *sql.Stmt) {
		if stmt != nil {
			if err := stmt.Close(); err != nil {
				errs = append(errs, err)
			}
		}
	}

	closeStmt(c.SaveUserDetails)
	closeStmt(c.GetUserId)
	closeStmt(c.GetUsername)
	closeStmt(c.GetFriends)
	closeStmt(c.CreateChat)
	closeStmt(c.GetChat)
	closeStmt(c.GetChats)
	closeStmt(c.UpdateChatState)
	closeStmt(c.SaveMessage)
	closeStmt(c.GetMessages)

	if len(errs) > 0 {
		return fmt.Errorf("error closing statements: %v", errs)
	}

	return nil

}
