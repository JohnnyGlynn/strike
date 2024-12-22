package db

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PreparedStatements struct {
	CreateUser string
	LoginUser  string
}

func PrepareStatements(ctx context.Context, dbpool *pgxpool.Pool) (*PreparedStatements, error) {

	poolConnection, err := dbpool.Acquire(ctx)
	if err != nil {
		log.Fatalf("failed to acquire connection from pool: %v", err)
		return nil, err
	}
	defer poolConnection.Release()

	statements := &PreparedStatements{
		CreateUser: "createUser",
		LoginUser:  "loginUser",
	}

	//Create User
	if _, err := poolConnection.Conn().Prepare(ctx, statements.CreateUser, "INSERT INTO userkeys (uname, publickey) VALUES ($1, $2)"); err != nil {
		return nil, err
	}

	//LoginUser
	if _, err := poolConnection.Conn().Prepare(ctx, statements.LoginUser, "SELECT EXISTS (SELECT 1 FROM userkeys WHERE uname = $1 AND publickey = $2)"); err != nil {
		return nil, err
	}

	return statements, nil

}
