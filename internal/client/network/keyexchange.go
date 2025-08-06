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

func InitiateKeyExchange(ctx context.Context, c *types.Client, target uuid.UUID) error {
	// make nonce
	nonce := make([]byte, 32)
	_, err := rand.Read(nonce)
	if err != nil {
		log.Printf("Error generating nonce: %v\n", err)
		return err
	}

	block, _ := pem.Decode(c.Identity.Keys["SigningPrivateKey"])
	if block == nil {
		return fmt.Errorf("failed to decode PEM block")
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		log.Print("failed to parse private key")
		return err
	}
	// ok if ed25519
	priv, ok := key.(ed25519.PrivateKey)
	if !ok {
		return fmt.Errorf("invalid ED25519 private key")
	}

	// signatures
	nonceSig := ed25519.Sign(priv, nonce)
	publicKeySig := ed25519.Sign(priv, c.Identity.Keys["EncryptionPublicKey"])

	sigs := [][]byte{nonceSig, publicKeySig}

	exchangeInfo := pb.KeyExchangeRequest{
		SenderUserId:   c.Identity.ID.String(),
		Target:         target.String(),
		CurvePublicKey: c.Identity.Keys["EncryptionPublicKey"],
		Nonce:          nonce,
		Signatures:     sigs,
	}

	payload := pb.StreamPayload{
		Target:  target.String(),
		Sender:  c.Identity.ID.String(),
		Payload: &pb.StreamPayload_KeyExchRequest{KeyExchRequest: &exchangeInfo},
		Info:    "Key Exchange initation payload",
	}

	resp, err := c.PBC.SendPayload(ctx, &payload)
	if err != nil {
		log.Printf("Error initiating key exchange: %v\n", err)
		return err
	}

	fmt.Printf("Key Exchange initiated: %v", resp.Success)

	return nil
}

func ReciprocateKeyExchange(ctx context.Context, c *types.Client, target uuid.UUID) error {
	// make nonce
	nonce := make([]byte, 32)
	_, err := rand.Read(nonce)
	if err != nil {
		log.Printf("Error generating nonce: %v\n", err)
		return err
	}

	block, _ := pem.Decode(c.Identity.Keys["SigningPrivateKey"])
	if block == nil {
		return fmt.Errorf("failed to decode PEM block")
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse private key")
	}
	// ok if ed25519
	priv, ok := key.(ed25519.PrivateKey)
	if !ok {
		return fmt.Errorf("invalid ED25519 private key")
	}

	// signatures
	nonceSig := ed25519.Sign(priv, nonce)
	publicKeySig := ed25519.Sign(priv, c.Identity.Keys["EncryptionPublicKey"])

	sigs := [][]byte{nonceSig, publicKeySig}

	exchangeInfo := pb.KeyExchangeResponse{
		ResponderUserId: c.Identity.ID.String(),
		CurvePublicKey:  c.Identity.Keys["EncryptionPublicKey"],
		Nonce:           nonce,
		Signatures:      sigs,
	}

	payload := pb.StreamPayload{
		Target:  target.String(),
		Sender:  c.Identity.ID.String(),
		Payload: &pb.StreamPayload_KeyExchResponse{KeyExchResponse: &exchangeInfo},
		Info:    "Key Exchange reciprocation payload",
	}

	resp, err := c.PBC.SendPayload(ctx, &payload)
	if err != nil {
		log.Printf("Error reciprocating key exchange: %v\n", err)
		return err
	}

	fmt.Printf("Key Exchange reciprocated: %v", resp.Success)

	return nil
}

func ConfirmKeyExchange(ctx context.Context, c *types.Client, target uuid.UUID, status bool) error {
	confirmation := pb.KeyExchangeConfirmation{
		Status:          status,
		ConfirmerUserId: c.Identity.ID.String(),
	}

	payload := pb.StreamPayload{
		Target:  target.String(),
		Sender:  c.Identity.ID.String(),
		Payload: &pb.StreamPayload_KeyExchConfirm{KeyExchConfirm: &confirmation},
		Info:    "Key Exchange confirmation paload",
	}

	resp, err := c.PBC.SendPayload(ctx, &payload)
	if err != nil {
		log.Printf("Error confirming key exchange: %v\n", err)
		return err
	}

	fmt.Printf("Key exchange confirmed: %v", resp.Success)

	return nil
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
