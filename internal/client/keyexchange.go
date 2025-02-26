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

func InitiateKeyExchange(ctx context.Context, c pb.StrikeClient, target string, username string, privateEDKey []byte, publicCurveKey []byte, chat *pb.Chat) {
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

	payload := pb.MessageStreamPayload{
		Target:  target,
		Payload: &pb.MessageStreamPayload_KeyExchRequest{KeyExchRequest: &exchangeInfo},
	}

	resp, err := c.SendPayload(ctx, &payload)
	if err != nil {
		log.Fatalf("Error initiating key exchange: %v", err)
	}

	fmt.Printf("Key Exchange initiated: %v", resp.Success)
}

func ReciprocateKeyExchange(ctx context.Context, c pb.StrikeClient, target string, username string, privateEDKey []byte, publicCurveKey []byte, chat *pb.Chat) {
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

	exchangeInfo := pb.KeyExchangeResponse{
		ChatId:          chat.Id,
		ResponderUserId: username,
		CurvePublicKey:  publicCurveKey,
		Nonce:           nonce,
		Signatures:      sigs,
	}

	payload := pb.MessageStreamPayload{
		Target:  target,
		Payload: &pb.MessageStreamPayload_KeyExchResponse{KeyExchResponse: &exchangeInfo},
	}

	resp, err := c.SendPayload(ctx, &payload)
	if err != nil {
		log.Fatalf("Error reciprocating key exchange: %v", err)
	}

	fmt.Printf("Key Exchange reciprocated: %v", resp.Success)
}

func ConfirmKeyExchange(ctx context.Context, c pb.StrikeClient, target string, status bool, chat *pb.Chat) {
	confirmation := pb.KeyExchangeConfirmation{
		ChatId: chat.Id,
		Status: status,
	}

	payload := pb.MessageStreamPayload{
		Target:  target,
		Payload: &pb.MessageStreamPayload_KeyExchConfirm{KeyExchConfirm: &confirmation},
	}

	resp, err := c.SendPayload(ctx, &payload)
	if err != nil {
		log.Fatalf("Error confirming key exchange: %v", err)
	}

	fmt.Printf("Key exchange confirmed: %v", resp.Success)
}

func ComputeSharedSecret(privateCurveKey []byte, inboundKey []byte) ([]byte, error) {
	// Validate our keys from []byte``
	private, err := ecdh.X25519().NewPrivateKey(privateCurveKey)
	if err != nil {
		return nil, fmt.Errorf("failed to validate key: %v", err)
	}

	public, err := ecdh.X25519().NewPublicKey(inboundKey)
	if err != nil {
		return nil, fmt.Errorf("failed to validate key: %v", err)
	}

	sharedSecret, err := private.ECDH(public)
	if err != nil {
		return nil, fmt.Errorf("failed to compute shared secret: %v", err)
	}

	return sharedSecret, nil
}

func VerifyEdSignatures(pubKey ed25519.PublicKey, nonce, CurvePublicKey []byte, sigs [][]byte) bool {
	if len(sigs) < 2 {
		return false
	}

	if !ed25519.Verify(pubKey, nonce, sigs[0]) {
		return false
	}

	return ed25519.Verify(pubKey, CurvePublicKey, sigs[1])
}
