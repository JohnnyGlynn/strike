package crypto

import (
	"bytes"
	"testing"

	"github.com/JohnnyGlynn/strike/internal/client/types"
	// "github.com/JohnnyGlynn/strike/internal/shared"
)

func TestDeriveKeys(t *testing.T) {
	type tparams struct {
		keylen int
		error  bool
	}

	cases := map[string]struct {
		c, c2        *types.Client
		sharedsecret []byte
		tparams      tparams
	}{
		"equal-output": {
			c:            &types.Client{State: &types.ClientState{Cache: types.Cache{}}},
			c2:           &types.Client{State: &types.ClientState{Cache: types.Cache{}}},
			sharedsecret: []byte("shared-secret"),
			tparams:      tparams{keylen: 32, error: false},
		},
		"no-secret": {
			c:            &types.Client{State: &types.ClientState{Cache: types.Cache{}}},
			c2:           nil,
			sharedsecret: []byte(""),
			tparams:      tparams{error: true},
		},
	}

	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			enc, hmac, err := DeriveKeys(tc.c, tc.sharedsecret)
			if tc.tparams.error {
				if err == nil {
					t.Fatal("error: no error")
				}
				return
			}

			if err != nil {
				t.Fatalf("derive error 1: %v", err)
			}

			if len(enc) != tc.tparams.keylen || len(hmac) != tc.tparams.keylen {
				t.Error("keys inconsistent: length not 32 byte")
			}

			if tc.c2 != nil {
				enc2, hmac2, err2 := DeriveKeys(tc.c2, tc.sharedsecret)

				if err2 == nil {
					t.Fatalf("derive error 2: %v", err2)
				}

				if !bytes.Equal(enc, enc2) || !bytes.Equal(hmac, hmac2) {
					t.Error("keys inconsistent: bytes not equal")
				}

			}

		})
	}
}
