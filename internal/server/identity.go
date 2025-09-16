package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

	"github.com/JohnnyGlynn/strike/internal/config"
	"github.com/JohnnyGlynn/strike/internal/keys"
)

const idFile = "identity.json"

type Identity struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func InitID(svrCfg config.ServerConfig, idCfg string) (*Identity, error) {

  //First run, check for keys
  if _, err := os.Stat(svrCfg.SigningPrivateKeyPath); os.IsNotExist(err){
    fmt.Println("No server identity found: Bootstrapping")

    err := keys.GenerateServerKeysAndCert()
    if err != nil {
      return nil, err
    }

    keyBytes, err := keys.GetKeyFromPath(svrCfg.SigningPublicKeyPath)
    if err != nil {
      return nil, err
    }

    fingerprint := DeriveServerID(keyBytes)

    id := &Identity{
      ID:   fingerprint,
      Name: "MAKE-CONFIGURABLE",
    }


    writes, err := json.Marshal(id)
    if err != nil {
      return nil, err
    }

    //Json path
    err = os.WriteFile(idCfg, writes, 0600)
    if err != nil {
      return nil, err
    }

    return id, nil

  }

	//If an existing id config has been created
	if _, err := os.Stat(idCfg); err == nil {
		file, err := os.ReadFile(idCfg)
		if err != nil {
			return nil, err
		}

		var id Identity
		if err := json.Unmarshal(file, &id); err != nil {
			return nil, err
		}
		return &id, nil
	}

  return &Identity{}, nil
}

func DeriveServerID(pub []byte) string {
	di := sha256.Sum256(pub)
	return hex.EncodeToString(di[:16])
}
