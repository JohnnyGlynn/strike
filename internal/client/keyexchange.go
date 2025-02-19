package client

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"log"

	pb "github.com/JohnnyGlynn/strike/msgdef/message"
)

func InitiateKeyExchange(ctx context.Context, c pb.StrikeClient, username string, privateEDKey []byte, publicCurveKey []byte, chat *pb.Chat) *pb.KeyExchangeResponse {
	// make nonce
	nonce := make([]byte, 32)
	_, err := rand.Read(nonce)
	if err != nil {
		log.Fatalf("Error generating nonce: %v", err)
	}

	// sign nonce
	signature := ed25519.Sign(ed25519.PrivateKey(privateEDKey), nonce)

	exchangeInfo := pb.KeyExchangeRequest{
		ChatId:           chat.Id,
		SenderUserId:     username, // TODO: Users need ID's
		CurvePublicKey:   publicCurveKey,
		Nonce:            nonce,
		Ed25519Signature: signature, // Sign more than nonce?
	}

	resp, err := c.InitiateKeyExchange(ctx, &exchangeInfo)
	if err != nil {
		log.Fatalf("Error initiating key exchange: %v", err)
	}
	return resp
}
