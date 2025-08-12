package server

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ServerDB struct {
	User struct {
		CreateUser string
		LoginUser  string
		GetUser    string
		SaltMine   string
	}

	Keys struct {
		GetPublicKeys    string
		CreatePublicKeys string
	}
}

const (

	//User
	sqlCreateUser = "INSERT INTO users (user_id, username, password_hash, salt) VALUES ($1, $2, $3, $4)"
	sqlLoginUser  = "SELECT password_hash FROM users WHERE username = $1"
	sqlGetUser    = "SELECT user_id FROM users WHERE username = $1"
	sqlSaltMine   = "SELECT salt FROM users WHERE username = $1"

	//Keys
	sqlCreatePublicKeys = "INSERT INTO user_keys (user_id, encryption_public_key, signing_public_key) VALUES ($1, $2, $3)"
	sqlGetPublicKeys    = "SELECT encryption_public_key, signing_public_key FROM user_keys WHERE user_id = $1"
)

func PrepareStatements(ctx context.Context, dbpool *pgxpool.Pool) (*ServerDB, error) {
	poolConnection, err := dbpool.Acquire(ctx)
	if err != nil {
		fmt.Printf("failed to acquire connection from pool: %v\n", err)
		return nil, err
	}
	defer poolConnection.Release()

	statements := &ServerDB{}

	statements.User.CreateUser = "createUser"
	statements.User.LoginUser = "loginUser"
	statements.User.GetUser = "getUser"
	statements.User.SaltMine = "saltMine"

	statements.Keys.CreatePublicKeys = "createPublicKeys"
	statements.Keys.GetPublicKeys = "GetPublicKeys"

	conn := poolConnection.Conn()

	pq := []struct {
		ps    string
		query string
	}{
		{statements.User.CreateUser, sqlCreateUser},
		{statements.User.LoginUser, sqlLoginUser},
		{statements.User.GetUser, sqlGetUser},
		{statements.User.SaltMine, sqlSaltMine},

		{statements.Keys.CreatePublicKeys, sqlCreatePublicKeys},
		{statements.Keys.GetPublicKeys, sqlGetPublicKeys},
	}

	for _, p := range pq {
		if _, err := conn.Prepare(ctx, p.ps, p.query); err != nil {
			return nil, fmt.Errorf("failed to prepare statement %s: %v", p.ps, err)
		}
	}

	return statements, nil
}
