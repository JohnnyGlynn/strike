package network

import (
	"context"
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log"

	"github.com/JohnnyGlynn/strike/internal/client/types"
	pb "github.com/JohnnyGlynn/strike/msgdef/message"
	"github.com/google/uuid"
)

func InitiateKeyExchange(ctx context.Context, c *types.ClientInfo, target uuid.UUID, chat *pb.Chat) {
	// make nonce
	nonce := make([]byte, 32)
	_, err := rand.Read(nonce)
	if err != nil {
		log.Fatalf("Error generating nonce: %v", err)
	}

	block, _ := pem.Decode(c.Keys["SigningPrivateKey"])
	if block == nil {
		log.Print("failed to decode PEM block")
		return
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		log.Print("failed to parse private key")
		return
	}
	// ok if ed25519
	priv, ok := key.(ed25519.PrivateKey)
	if !ok {
		log.Print("invalid ED25519 private key")
		return
	}

	// signatures
	nonceSig := ed25519.Sign(priv, nonce)
	publicKeySig := ed25519.Sign(priv, c.Keys["EncryptionPublicKey"])

	sigs := [][]byte{nonceSig, publicKeySig}

	c.Cache.Chats[uuid.MustParse(chat.Id)].State = pb.Chat_KEY_EXCHANGE_PENDING

	exchangeInfo := pb.KeyExchangeRequest{
		ChatId:         chat.Id,
		SenderUserId:   c.UserID.String(),
		Target:         target.String(),
		CurvePublicKey: c.Keys["EncryptionPublicKey"],
		Nonce:          nonce,
		Signatures:     sigs,
	}

	fmt.Printf("Target UUID: %v", target.String())

	payload := pb.StreamPayload{
		Target:  target.String(),
		Sender:  c.UserID.String(),
		Payload: &pb.StreamPayload_KeyExchRequest{KeyExchRequest: &exchangeInfo},
		Info:    "Key Exchange initation payload",
	}

	resp, err := c.Pbclient.SendPayload(ctx, &payload)
	if err != nil {
		log.Fatalf("Error initiating key exchange: %v", err)
	}

	fmt.Printf("Key Exchange initiated: %v", resp.Success)
}

func ReciprocateKeyExchange(ctx context.Context, c *types.ClientInfo, target uuid.UUID, chat *pb.Chat) {
	// make nonce
	nonce := make([]byte, 32)
	_, err := rand.Read(nonce)
	if err != nil {
		log.Fatalf("Error generating nonce: %v", err)
	}

	block, _ := pem.Decode(c.Keys["SigningPrivateKey"])
	if block == nil {
		log.Print("failed to decode PEM block")
		return
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		log.Print("failed to parse private key")
		return
	}
	// ok if ed25519
	priv, ok := key.(ed25519.PrivateKey)
	if !ok {
		log.Print("invalid ED25519 private key")
		return
	}

	// signatures
	nonceSig := ed25519.Sign(priv, nonce)
	publicKeySig := ed25519.Sign(priv, c.Keys["EncryptionPublicKey"])

	sigs := [][]byte{nonceSig, publicKeySig}

	exchangeInfo := pb.KeyExchangeResponse{
		ChatId: chat.Id,
		// TODO:UUID NOT USERNAME, REAL uuid
		ResponderUserId: c.UserID.String(),
		CurvePublicKey:  c.Keys["EncryptionPublicKey"],
		Nonce:           nonce,
		Signatures:      sigs,
	}

	payload := pb.StreamPayload{
		Target:  target.String(),
		Sender:  c.UserID.String(),
		Payload: &pb.StreamPayload_KeyExchResponse{KeyExchResponse: &exchangeInfo},
		Info:    "Key Exchange reciprocation payload",
	}

	resp, err := c.Pbclient.SendPayload(ctx, &payload)
	if err != nil {
		log.Fatalf("Error reciprocating key exchange: %v", err)
	}

	fmt.Printf("Key Exchange reciprocated: %v", resp.Success)
}

func ConfirmKeyExchange(ctx context.Context, c *types.ClientInfo, target uuid.UUID, status bool, chat *pb.Chat) {
	confirmation := pb.KeyExchangeConfirmation{
		ChatId:          chat.Id,
		Status:          status,
		ConfirmerUserId: c.UserID.String(),
	}

	payload := pb.StreamPayload{
		Target:  target.String(),
		Sender:  c.UserID.String(),
		Payload: &pb.StreamPayload_KeyExchConfirm{KeyExchConfirm: &confirmation},
		Info:    "Key Exchange confirmation paload",
	}

	c.Cache.Chats[uuid.MustParse(chat.Id)] = chat

	resp, err := c.Pbclient.SendPayload(ctx, &payload)
	if err != nil {
		log.Fatalf("Error confirming key exchange: %v", err)
	}

	fmt.Printf("Key exchange confirmed: %v", resp.Success)
}

func ComputeSharedSecret(privateCurveKey []byte, inboundKey []byte) ([]byte, error) {
	block, _ := pem.Decode(privateCurveKey)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	// Validate our keys from []byte``
	private, err := ecdh.X25519().NewPrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to validate key: %v", err)
	}

	pubblock, _ := pem.Decode(inboundKey)

	public, err := ecdh.X25519().NewPublicKey(pubblock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to validate key: %v", err)
	}

	sharedSecret, err := private.ECDH(public)
	if err != nil {
		log.Printf("failed to carry out diffie hellman key exchange: %v", err)
		return nil, fmt.Errorf("failed to compute shared secret: %v", err)
	}

	return sharedSecret, nil
}
