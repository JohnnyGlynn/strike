package client

import (
	"context"
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
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

	// signatures
	nonceSig := ed25519.Sign(ed25519.PrivateKey(privateEDKey), nonce)
	publicKeySig := ed25519.Sign(ed25519.PrivateKey(privateEDKey), publicCurveKey)

	sigs := [][]byte{nonceSig, publicKeySig}

	exchangeInfo := pb.KeyExchangeRequest{
		ChatId:         chat.Id,
		SenderUserId:   username, // TODO: Users need ID's
		CurvePublicKey: publicCurveKey,
		Nonce:          nonce,
		Signatures:     sigs,
	}

	resp, err := c.InitiateKeyExchange(ctx, &exchangeInfo)
	if err != nil {
		log.Fatalf("Error initiating key exchange: %v", err)
	}
	return resp
}

func ComputeSharedSecret(privateCurveKey []byte, inboundKeyExchange *pb.KeyExchangeResponse) ([]byte, error) {
	// Validate our keys from []byte``
	private, err := ecdh.X25519().NewPrivateKey(privateCurveKey)
	if err != nil {
		return nil, fmt.Errorf("failed to validate key: %v", err)
	}

	public, err := ecdh.X25519().NewPublicKey(inboundKeyExchange.CurvePublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to validate key: %v", err)
	}

	sharedSecret, err := private.ECDH(public)
	if err != nil {
		return nil, fmt.Errorf("failed to compute shared secret: %v", err)
	}

	return sharedSecret, nil
}
