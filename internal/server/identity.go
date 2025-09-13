package server

import (
	"crypto/sha256"
	"encoding/hex"

	// "github.com/JohnnyGlynn/strike/internal/config"
)

const idFile = "identity.json"

type Identity struct {
  ID         string `json:"id"`
  Name string `json:"name"`
}

// func InitID(svrCfg config.ServerConfig) (*Identity, error) {

// }

func DeriveServerID(pub []byte) string {
	di := sha256.Sum256(pub)
	return hex.EncodeToString(di[:16])
}
