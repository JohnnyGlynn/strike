package crypto

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/rand"
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
		"no-cache": {
			c:            &types.Client{State: &types.ClientState{}},
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

			if !bytes.Equal(enc, tc.c.State.Cache.CurrentChat.EncKey) || !bytes.Equal(hmac, tc.c.State.Cache.CurrentChat.HmacKey) {
				t.Fatalf("error: cache values !=")
			}

			if tc.c2 != nil {
				enc2, hmac2, err2 := DeriveKeys(tc.c2, tc.sharedsecret)

				if err2 != nil {
					t.Fatalf("derive error 2: %v", err2)
				}

				if !bytes.Equal(enc, enc2) || !bytes.Equal(hmac, hmac2) {
					t.Error("keys inconsistent: bytes not equal")
				}

			}

		})
	}
}

func TestEncryptDecrypt(t *testing.T) {
	type tparams struct {
		error bool
	}

	cases := map[string]struct {
		c       *types.Client
		message []byte
		tparams tparams
	}{
		"valid": {
			c: &types.Client{
				State: &types.ClientState{
					Cache: types.Cache{
						CurrentChat: types.ChatSession{
							EncKey: []byte("0123456789abcdef0123456789abcdef"),
						},
					},
				},
			},
			message: []byte("message to be sealed"),
			tparams: tparams{error: false},
		},
		"short-key": {
			c: &types.Client{
				State: &types.ClientState{
					Cache: types.Cache{
						CurrentChat: types.ChatSession{
							EncKey: []byte("short"),
						},
					},
				},
			},
			message: []byte("message to be sealed"),
			tparams: tparams{error: true},
		},
	}

	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			sealedMessage, err := Encrypt(tc.c, tc.message)
			if tc.tparams.error {
				if err == nil {
					t.Fatal("error: no error")
				}
				return
			}

			if err != nil {
				t.Fatalf("encrypt error: %v", err)
			}

			block, _ := aes.NewCipher(tc.c.State.Cache.CurrentChat.EncKey)
			gcm, _ := cipher.NewGCM(block)

			packedLen := gcm.NonceSize() + len(tc.message) + gcm.Overhead()
			if len(sealedMessage) != packedLen {
				t.Errorf("sealed message too short")
			}

			plaintext, err := Decrypt(tc.c, sealedMessage)
			if err != nil {
				t.Fatalf("decpryt error: %v", err)
			}

			if !bytes.Equal(plaintext, tc.message) {
				t.Errorf("message mismatch after decrypt")
			}
		})
	}
}

func TestVerifyEdSignatures(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal()
	}

	nonce := []byte("nonce")
	curve := []byte("curve-public")

	sigNonce := ed25519.Sign(priv, nonce)
	sigCurve := ed25519.Sign(priv, curve)

	cases := map[string]struct {
		pub   ed25519.PublicKey
		nonce []byte
		curve []byte
		sigs  [][]byte
		valid bool
	}{
		"valid": {
			pub:   pub,
			nonce: nonce,
			curve: curve,
			sigs:  [][]byte{sigNonce, sigCurve},
			valid: true,
		},
		"1-sig": {
			pub:   pub,
			nonce: nonce,
			curve: curve,
			sigs:  [][]byte{sigCurve},
			valid: false,
		},
		"bad-nonce": {
			pub:   pub,
			nonce: []byte("incorrect"),
			curve: curve,
			sigs:  [][]byte{sigNonce, sigCurve},
			valid: false,
		},
		"bad-curve": {
			pub:   pub,
			nonce: nonce,
			curve: []byte("incorrect-key"),
			sigs:  [][]byte{sigNonce, sigCurve},
			valid: false,
		},
	}

	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ret := VerifyEdSignatures(tc.pub, tc.nonce, tc.curve, tc.sigs)
			if ret != tc.valid {
				t.Errorf("VerifyEdSignatures() = %v, wanted %v", ret, tc.valid)
			}

		})
	}

}
