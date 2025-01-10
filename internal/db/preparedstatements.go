package db

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PreparedStatements struct {
	CreateUser  string
	LoginUser   string
	GetUserKeys string
}

func PrepareStatements(ctx context.Context, dbpool *pgxpool.Pool) (*PreparedStatements, error) {

	poolConnection, err := dbpool.Acquire(ctx)
	if err != nil {
		log.Fatalf("failed to acquire connection from pool: %v", err)
		return nil, err
	}
	defer poolConnection.Release()

	statements := &PreparedStatements{
		CreateUser:  "createUser",
		LoginUser:   "loginUser",
		GetUserKeys: "getUserKeys",
	}

	//Create User
	if _, err := poolConnection.Conn().Prepare(ctx, statements.CreateUser, "INSERT INTO users (username, encryption_key, signing_key) VALUES ($1, $2, $3)"); err != nil {
		return nil, err
	}

	//LoginUser
	if _, err := poolConnection.Conn().Prepare(ctx, statements.LoginUser, "SELECT EXISTS (SELECT 1 FROM users WHERE username = $1 AND encryption_key = $2)"); err != nil {
		return nil, err
	}

	//Get keys to start chat
	if _, err := poolConnection.Conn().Prepare(ctx, statements.GetUserKeys, "SELECT encryption_key, signing_key FROM users WHERE username = $1;"); err != nil {
		return nil, err
	}

	return statements, nil

}
