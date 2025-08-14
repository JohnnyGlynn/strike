package crypto

import (
	"bytes"
	"testing"

	"github.com/JohnnyGlynn/strike/internal/client/types"
)


func TestDeriveKeys (t *testing.T) {
  sharedsecret := []byte("shared-secret")
  c := types.Client{
    State: &types.ClientState{
      Cache: types.Cache{},
    },
  }

  enc, hmac, err := DeriveKeys(&c, sharedsecret)
  if err != nil {
    t.Fatalf("DeriveKeys failed: %v", err)
  }

  c2 := types.Client{
    State: &types.ClientState{
      Cache: types.Cache{},
    },
  }

  enc2, hmac2, err := DeriveKeys(&c2, sharedsecret)
  if err != nil {
    t.Fatalf("DeriveKeys failed: %v", err)//fail now
  }

  if !bytes.Equal(enc, enc2) || !bytes.Equal(hmac, hmac2) {
    t.Error("keys inconsistent: bytes not equal")
  }

  if len(enc) != 32 || len(enc2) != 32 {
    t.Error("keys inconsistent: length not 32 byte")
  } 

}
