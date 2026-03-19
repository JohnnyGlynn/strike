package server

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/JohnnyGlynn/strike/internal/config"
	"github.com/JohnnyGlynn/strike/internal/keys"
)

type Identity struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func InitID(svrCfg config.ServerConfig, idCfg string) (*Identity, error) {

	//First run, check for keys
	if _, err := os.Stat(svrCfg.SigningPrivateKeyPath); os.IsNotExist(err) {
		fmt.Println("No server identity found: Bootstrapping")

		// Use the directory containing the expected private key path as output
		keyDir := filepath.Dir(svrCfg.SigningPrivateKeyPath)
		err := keys.GenerateServerKeysAndCert(keyDir)
		if err != nil {
			return nil, err
		}
	}

	keyBytes, err := keys.GetKeyFromPath(svrCfg.SigningPublicKeyPath)
	if err != nil {
		return nil, err
	}

	fingerprint := keys.DeriveID(keyBytes)

	if _, err := os.Stat(idCfg); err == nil {
		file, err := os.ReadFile(idCfg)
		if err != nil {
			return nil, err
		}

		var id Identity
		if err := json.Unmarshal(file, &id); err != nil {
			return nil, err
		}

		if id.ID != fingerprint {
			return nil, fmt.Errorf("identity mismatch")
		}
		return &id, nil
	}

	id := &Identity{
		ID:   fingerprint,
		Name: svrCfg.Name,
	}

	writes, err := json.Marshal(id)
	if err != nil {
		return nil, err
	}

	err = os.WriteFile(idCfg, writes, 0600)
	if err != nil {
		return nil, err
	}

	return id, nil
}

// DeriveServerID wraps keys.DeriveID for use within the server package
func DeriveServerID(pub []byte) string {
	return keys.DeriveID(pub)
}
