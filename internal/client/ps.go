package client

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/JohnnyGlynn/strike/internal/client/types"
)

const (
	//Friends
	sqlGetFriends      = `SELECT * FROM addressbook`
	sqlSaveUserDetails = `
    INSERT INTO addressbook (user_id, username, enc_pkey, sig_pkey) 
    VALUES (?, ?, ?, ?) ON CONFLICT(user_id) DO UPDATE SET 
    username=excluded.username, 
    enc_pkey=excluded.enc_pkey, 
    sig_pkey=excluded.sig_pkey
  `
	sqlGetUserId    = "SELECT user_id FROM addressbook WHERE username = ?"
	sqlGetUser      = "SELECT * FROM addressbook WHERE username = ?"
	sqlGetKeyEx     = "SELECT keyex FROM addressbook WHERE user_id = ?"
	sqlConfirmKeyEx = "UPDATE addressbook SET keyex = ? WHERE user_id = ?"

	//ID
	sqlGetID  = "SELECT * FROM identity WHERE username = ?"
	sqlGetUID = "SELECT user_id FROM identity WHERE username = ?"
	sqlSaveID = "INSERT INTO identity (user_id, username, enc_pkey, sig_pkey) VALUES (?, ?, ?, ?)"

	//Messages
	sqlSaveMessage = "INSERT INTO messages (id, friendId, direction, content, timestamp) VALUES (?, ?, ?, ?, ?)"
	sqlGetMessages = "SELECT * FROM messages WHERE friendId = ? ORDER BY timestamp ASC, id ASC"

	//Friend Requests
	sqlSaveFriendRequest   = "INSERT INTO friendrequests (friendId, username, enc_pkey, sig_pkey, direction) VALUES (?, ?, ?, ?, ?)"
	sqlGetFriendRequests   = "SELECT * FROM friendrequests"
	sqlDeleteFriendRequest = "DELETE FROM friendrequests WHERE friendId = ?"
)

func PrepareStatements(ctx context.Context, db *sql.DB) (*types.ClientDB, error) {
	statements := &types.ClientDB{}

	pq := []struct {
		ps    **sql.Stmt
		query string
	}{
		{&statements.Friends.GetFriends, sqlGetFriends},
		{&statements.Friends.SaveUserDetails, sqlSaveUserDetails},
		{&statements.Friends.GetUserId, sqlGetUserId},
		{&statements.Friends.GetUser, sqlGetUser},
		{&statements.Friends.GetKeyEx, sqlGetKeyEx},
		{&statements.Friends.ConfirmKeyEx, sqlConfirmKeyEx},
		{&statements.ID.GetID, sqlGetID},
		{&statements.ID.GetUID, sqlGetUID},
		{&statements.ID.SaveID, sqlSaveID},
		{&statements.Messages.SaveMessage, sqlSaveMessage},
		{&statements.Messages.GetMessages, sqlGetMessages},
		{&statements.FriendRequest.SaveFriendRequest, sqlSaveFriendRequest},
		{&statements.FriendRequest.GetFriendRequests, sqlGetFriendRequests},
		{&statements.FriendRequest.DeleteFriendRequest, sqlDeleteFriendRequest},
	}

	for _, p := range pq {
		stmt, err := db.PrepareContext(ctx, p.query)
		if err != nil {
			return nil, err
		}
		*p.ps = stmt
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

	statements := []*sql.Stmt{
		// Friends
		c.Friends.SaveUserDetails,
		c.Friends.GetFriends,
		c.Friends.GetKeyEx,
		c.Friends.ConfirmKeyEx,
		c.Friends.GetUserId,
		c.Friends.GetUser,

		// Identity
		c.ID.GetID,
		c.ID.GetUID,
		c.ID.SaveID,

		// Messages
		c.Messages.SaveMessage,
		c.Messages.GetMessages,

		// Friend requests
		c.FriendRequest.SaveFriendRequest,
		c.FriendRequest.GetFriendRequests,
		c.FriendRequest.DeleteFriendRequest,
	}

	for _, stmt := range statements {
		closeStmt(stmt)
	}

	if len(errs) > 0 {
		return fmt.Errorf("error closing statements: %v", errs)
	}

	return nil

}
