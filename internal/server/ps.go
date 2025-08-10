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

	// User
	if _, err := conn.Prepare(ctx, statements.User.CreateUser, "INSERT INTO users (user_id, username, password_hash, salt) VALUES ($1, $2, $3, $4)"); err != nil {
		return nil, err
	}

	if _, err := conn.Prepare(ctx, statements.User.GetUser, "SELECT user_id FROM users WHERE username = $1;"); err != nil {
		return nil, err
	}

	if _, err := conn.Prepare(ctx, statements.User.LoginUser, "SELECT password_hash FROM users WHERE username = $1"); err != nil {
		return nil, err
	}

	if _, err := conn.Prepare(ctx, statements.User.SaltMine, "SELECT salt FROM users WHERE username = $1"); err != nil {
		return nil, err
	}

	// Keys
	if _, err := conn.Prepare(ctx, statements.Keys.CreatePublicKeys, "INSERT INTO user_keys (user_id, encryption_public_key, signing_public_key) VALUES ($1, $2, $3)"); err != nil {
		return nil, err
	}

	if _, err := conn.Prepare(ctx, statements.Keys.GetPublicKeys, "SELECT encryption_public_key, signing_public_key FROM user_keys  WHERE user_id = $1;"); err != nil {
		return nil, err
	}

	return statements, nil
}
