package client

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/JohnnyGlynn/strike/internal/client/types"
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

	if statements.GetUser, err = db.PrepareContext(ctx, `SELECT * FROM addressbook WHERE username = ?;`); err != nil {
		return nil, err
	}

	if statements.GetID, err = db.PrepareContext(ctx, `SELECT * FROM identity WHERE username = ?;`); err != nil {
		return nil, err
	}

	if statements.SaveID, err = db.PrepareContext(ctx, `INSERT INTO identity (user_id, username, enc_pkey, sig_pkey) VALUES (?, ?, ?, ?);`); err != nil {
		return nil, err
	}

	if statements.GetFriends, err = db.PrepareContext(ctx, `SELECT * FROM addressbook`); err != nil {
		return nil, err
	}

	// Key exchange check
	if statements.GetKeyEx, err = db.PrepareContext(ctx, `SELECT keyex FROM addressbook WHERE user_id = ?;`); err != nil {
		return nil, err
	}

	// Key exchange confirm
	if statements.ConfirmKeyEx, err = db.PrepareContext(ctx, `UPDATE addressbook SET keyex = ? WHERE user_id = ?`); err != nil {
		return nil, err
	}

	// Insert Message - Insert message bound by chat
	if statements.SaveMessage, err = db.PrepareContext(ctx, `INSERT INTO messages (id, friendId, direction, content, timestamp) VALUES (?, ?, ?, ?, ?)`); err != nil {
		return nil, err
	}

	// get all messages
	if statements.GetMessages, err = db.PrepareContext(ctx, `SELECT * FROM messages WHERE friendId = ? ORDER BY timestamp ASC, id ASC;`); err != nil {
		return nil, err
	}

	if statements.SaveFriendRequest, err = db.PrepareContext(ctx, `INSERT INTO friendrequests (friendId, username, enc_pkey, sig_pkey, direction) VALUES (?, ?, ?, ?, ?)`); err != nil {
		return nil, err
	}

	if statements.GetFriendRequests, err = db.PrepareContext(ctx, `SELECT * FROM friendrequests`); err != nil {
		return nil, err
	}

	if statements.DeleteFriendRequest, err = db.PrepareContext(ctx, `DELETE FROM friendrequests WHERE friendId = ?`); err != nil {
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
	closeStmt(c.GetUser)
	closeStmt(c.GetID)
	closeStmt(c.SaveID)
	closeStmt(c.GetFriends)
	closeStmt(c.GetKeyEx)
	closeStmt(c.ConfirmKeyEx)
	closeStmt(c.SaveMessage)
	closeStmt(c.GetMessages)
	closeStmt(c.SaveFriendRequest)
	closeStmt(c.GetFriendRequests)
	closeStmt(c.DeleteFriendRequest)

	if len(errs) > 0 {
		return fmt.Errorf("error closing statements: %v", errs)
	}

	return nil

}
