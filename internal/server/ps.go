package server

import (
	"context"

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

// InitStatements stores the SQL strings directly.
// pgxpool handles automatic statement caching per connection,
// so explicit Prepare calls are not needed.
func InitStatements(ctx context.Context, dbpool *pgxpool.Pool) (*ServerDB, error) {
	// Verify DB is reachable
	if err := dbpool.Ping(ctx); err != nil {
		return nil, err
	}

	return &ServerDB{
		User: struct {
			CreateUser string
			LoginUser  string
			GetUser    string
			SaltMine   string
		}{
			CreateUser: "INSERT INTO users (user_id, username, password_hash, salt) VALUES ($1, $2, $3, $4)",
			LoginUser:  "SELECT password_hash FROM users WHERE username = $1",
			GetUser:    "SELECT user_id FROM users WHERE username = $1",
			SaltMine:   "SELECT salt FROM users WHERE username = $1",
		},
		Keys: struct {
			GetPublicKeys    string
			CreatePublicKeys string
		}{
			GetPublicKeys:    "SELECT encryption_public_key, signing_public_key FROM user_keys WHERE user_id = $1",
			CreatePublicKeys: "INSERT INTO user_keys (user_id, encryption_public_key, signing_public_key) VALUES ($1, $2, $3)",
		},
	}, nil
}
