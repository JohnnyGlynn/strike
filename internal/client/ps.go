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

	// Friends
	if statements.Friends.GetFriends, err = db.PrepareContext(ctx, `SELECT * FROM addressbook`); err != nil {
		return nil, err
	}

	if statements.Friends.SaveUserDetails, err = db.PrepareContext(ctx, `INSERT INTO addressbook (user_id, username, enc_pkey, sig_pkey) VALUES (?, ?, ?, ?) ON CONFLICT(user_id) DO UPDATE SET username=excluded.username, enc_pkey=excluded.enc_pkey, sig_pkey=excluded.sig_pkey;`); err != nil {
		return nil, err
	}

	// Get User Id
	if statements.Friends.GetUserId, err = db.PrepareContext(ctx, `SELECT user_id FROM addressbook WHERE username = ?;`); err != nil {
		return nil, err
	}

	if statements.Friends.GetUser, err = db.PrepareContext(ctx, `SELECT * FROM addressbook WHERE username = ?;`); err != nil {
		return nil, err
	}

	// Key exchange check
	if statements.Friends.GetKeyEx, err = db.PrepareContext(ctx, `SELECT keyex FROM addressbook WHERE user_id = ?;`); err != nil {
		return nil, err
	}

	// Key exchange confirm
	if statements.Friends.ConfirmKeyEx, err = db.PrepareContext(ctx, `UPDATE addressbook SET keyex = ? WHERE user_id = ?`); err != nil {
		return nil, err
	}

	// ID
	if statements.ID.GetID, err = db.PrepareContext(ctx, `SELECT * FROM identity WHERE username = ?;`); err != nil {
		return nil, err
	}

	if statements.ID.GetUID, err = db.PrepareContext(ctx, `SELECT user_id FROM identity WHERE username = ?;`); err != nil {
		return nil, err
	}

	if statements.ID.SaveID, err = db.PrepareContext(ctx, `INSERT INTO identity (user_id, username, enc_pkey, sig_pkey) VALUES (?, ?, ?, ?);`); err != nil {
		return nil, err
	}

	// Messages
	if statements.Messages.SaveMessage, err = db.PrepareContext(ctx, `INSERT INTO messages (id, friendId, direction, content, timestamp) VALUES (?, ?, ?, ?, ?)`); err != nil {
		return nil, err
	}

	if statements.Messages.GetMessages, err = db.PrepareContext(ctx, `SELECT * FROM messages WHERE friendId = ? ORDER BY timestamp ASC, id ASC;`); err != nil {
		return nil, err
	}

	// Friend requests
	if statements.FriendRequest.SaveFriendRequest, err = db.PrepareContext(ctx, `INSERT INTO friendrequests (friendId, username, enc_pkey, sig_pkey, direction) VALUES (?, ?, ?, ?, ?)`); err != nil {
		return nil, err
	}

	if statements.FriendRequest.GetFriendRequests, err = db.PrepareContext(ctx, `SELECT * FROM friendrequests`); err != nil {
		return nil, err
	}

	if statements.FriendRequest.DeleteFriendRequest, err = db.PrepareContext(ctx, `DELETE FROM friendrequests WHERE friendId = ?`); err != nil {
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

	// Friends
	closeStmt(c.Friends.SaveUserDetails)
	closeStmt(c.Friends.GetFriends)
	closeStmt(c.Friends.GetKeyEx)
	closeStmt(c.Friends.ConfirmKeyEx)
	closeStmt(c.Friends.GetUserId)
	closeStmt(c.Friends.GetUser)

	// Identity
	closeStmt(c.ID.GetID)
	closeStmt(c.ID.GetUID)
	closeStmt(c.ID.SaveID)

	// Messages
	closeStmt(c.Messages.SaveMessage)
	closeStmt(c.Messages.GetMessages)

	// Friend requests
	closeStmt(c.FriendRequest.SaveFriendRequest)
	closeStmt(c.FriendRequest.GetFriendRequests)
	closeStmt(c.FriendRequest.DeleteFriendRequest)

	if len(errs) > 0 {
		return fmt.Errorf("error closing statements: %v", errs)
	}

	return nil

}
