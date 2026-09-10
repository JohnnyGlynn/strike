package server

import (
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"

	"github.com/JohnnyGlynn/strike/internal/keys"
	"github.com/JohnnyGlynn/strike/internal/server/types"
	"gopkg.in/yaml.v3"

	"github.com/google/uuid"
	"google.golang.org/grpc/credentials"
	grpcpeer "google.golang.org/grpc/peer"

	pb "github.com/JohnnyGlynn/strike/msgdef/federation"
)

type FederationOrchestrator struct {
	pb.UnimplementedFederationServer

	strike *StrikeServer
}

func NewFederationOrchestrator(s *StrikeServer) *FederationOrchestrator {
	return &FederationOrchestrator{
		strike: s,
	}
}

// peerCertPubKey pulls the ed25519 public key out of the TLS certificate the
// connecting peer presented for this RPC, if any.
func peerCertPubKey(ctx context.Context) (ed25519.PublicKey, bool) {
	p, ok := grpcpeer.FromContext(ctx)
	if !ok || p.AuthInfo == nil {
		return nil, false
	}

	tlsInfo, ok := p.AuthInfo.(credentials.TLSInfo)
	if !ok || len(tlsInfo.State.PeerCertificates) == 0 {
		return nil, false
	}

	pub, ok := tlsInfo.State.PeerCertificates[0].PublicKey.(ed25519.PublicKey)
	if !ok {
		return nil, false
	}
	return pub, true
}

func (fo *FederationOrchestrator) Handshake(
	ctx context.Context,
	req *pb.HandshakeReq,
) (*pb.HandshakeAck, error) {

	if req.ServerId == "" {
		return &pb.HandshakeAck{
			Ok:      false,
			Message: "missing server_id",
		}, nil
	}

	// The transport already only accepts connections from a known (pinned or
	// CA-chained) peer — see LoadFederationTLSConfig. Here we bind the
	// specific identity this caller claims (ServerId/ServerName) to the
	// specific pinned peer whose key was actually presented, so peer A's
	// certificate can't be used to claim to be peer B.
	presented, ok := peerCertPubKey(ctx)
	if !ok {
		return &pb.HandshakeAck{
			Ok:      false,
			Message: "no verifiable peer certificate",
		}, nil
	}

	matched := false
	for _, p := range fo.strike.PeerMgr.Peers() {
		if len(p.PubKey) == 0 || !presented.Equal(p.PubKey) {
			continue
		}
		if p.Name == req.ServerName || p.ID.String() == req.ServerId {
			matched = true
			break
		}
	}
	if !matched {
		fmt.Printf("federation: rejecting handshake — %s (%s) does not match a known peer's pinned key\n", req.ServerName, req.ServerId)
		return &pb.HandshakeAck{
			Ok:      false,
			Message: "server identity does not match a known peer",
		}, nil
	}

	fmt.Printf("federation handshake from server %s (%s) — identity verified\n", req.ServerName, req.ServerId)

	return &pb.HandshakeAck{
		Ok:       true,
		ServerId: fo.strike.ID.String(),
		Message:  "handshake accepted",
	}, nil
}

func (fo *FederationOrchestrator) Relay(
	ctx context.Context,
	rp *pb.RelayPayload,
) (*pb.RelayAck, error) {

	if rp == nil || rp.Sender == nil || rp.Recipient == nil {
		return &pb.RelayAck{
			EnvelopeId: rp.GetEnvelopeId(),
			Accepted:   false,
			Info:       "invalid payload",
		}, nil
	}

	senderID, err := uuid.Parse(rp.Sender.UInfo.UserId)
	if err == nil {
		fo.strike.UpdateRemotePresence(senderID, rp.OriginServer)
	}

	if err := fo.strike.EnqueueFederated(ctx, rp); err != nil {
		return &pb.RelayAck{
			EnvelopeId: rp.EnvelopeId,
			Accepted:   false,
			Info:       err.Error(),
		}, nil
	}

	return &pb.RelayAck{
		EnvelopeId: rp.EnvelopeId,
		Accepted:   true,
		Info:       "accepted",
	}, nil
}

func (fo *FederationOrchestrator) UserLookup(
	ctx context.Context,
	req *pb.UserLookupReq,
) (*pb.UserLookupResp, error) {

	if req.Username == "" {
		return &pb.UserLookupResp{Found: false}, nil
	}

	uInfo, err := fo.strike.localUserLookup(ctx, req.Username)
	if err != nil {
		return &pb.UserLookupResp{Found: false}, nil
	}

	if uInfo == nil {
		return &pb.UserLookupResp{Found: false}, nil
	}

	return &pb.UserLookupResp{
		Found:    true,
		UserInfo: uInfo,
		Domain:   fo.strike.Name,
	}, nil
}

func LoadPeers(path string) ([]types.PeerConfig, error) {
	peerConfig, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("federation: no peers file at %s — starting with zero peers\n", path)
			return nil, nil
		}
		return nil, err
	}

	var cfg types.FederationConfig
	if err := yaml.Unmarshal(peerConfig, &cfg); err != nil {
		return nil, err
	}

	// Decode base64 SPKI public keys
	for i := range cfg.Peers {
		raw, err := base64.StdEncoding.DecodeString(cfg.Peers[i].RawKey)
		if err != nil {
			return nil, fmt.Errorf("failed to decode pubkey for peer %s: %v", cfg.Peers[i].Name, err)
		}

		parsed, err := x509.ParsePKIXPublicKey(raw)
		if err != nil {
			return nil, fmt.Errorf("failed to parse pubkey for peer %s: %v", cfg.Peers[i].Name, err)
		}

		pubKey, ok := parsed.(ed25519.PublicKey)
		if !ok {
			return nil, fmt.Errorf("peer %s pubkey is not ed25519", cfg.Peers[i].Name)
		}

		cfg.Peers[i].PubKey = pubKey
	}

	fmt.Println("Available peers")
	for _, p := range cfg.Peers {
		fmt.Printf("%s@%s\n", p.Name, p.Address)
	}

	return cfg.Peers, nil
}

// ExportPeerBlock returns a small, pasteable YAML document describing this
// server as a federation peer — its derived ID, a chosen display name, the
// address it's reachable at, and its public signing key. A friend running
// their own Strike server records it with AddPeerToFile (see --add-peer) to
// start trusting this server. No CA and no signing step: adding a peer is
// just recording their key, the same known_hosts model as an SSH
// authorized_keys entry.
func ExportPeerBlock(name, addr, pubKeyPath string) (string, error) {
	pubPEM, err := os.ReadFile(pubKeyPath)
	if err != nil {
		return "", fmt.Errorf("failed to read public key at %s: %v", pubKeyPath, err)
	}

	block, _ := pem.Decode(pubPEM)
	if block == nil {
		return "", fmt.Errorf("failed to decode PEM at %s", pubKeyPath)
	}

	entry := types.PeerConfig{
		ID:      uuid.MustParse(keys.DeriveID(pubPEM)),
		Name:    name,
		Address: addr,
		RawKey:  base64.StdEncoding.EncodeToString(block.Bytes),
	}

	out, err := yaml.Marshal(entry)
	if err != nil {
		return "", fmt.Errorf("failed to marshal peer entry: %v", err)
	}

	return string(out), nil
}

// AddPeerToFile parses a single peer-card YAML document (as produced by
// ExportPeerBlock) and appends it to the federation peers file at path,
// creating the file if it doesn't exist yet. Fails if a peer with the same
// ID or name is already present, so re-adding the same card is a no-op
// error rather than a silent duplicate.
func AddPeerToFile(path string, card []byte) error {
	var entry types.PeerConfig
	if err := yaml.Unmarshal(card, &entry); err != nil {
		return fmt.Errorf("failed to parse peer card: %v", err)
	}
	if entry.Name == "" || entry.Address == "" || entry.RawKey == "" {
		return fmt.Errorf("peer card is missing name, addr, or pubkey")
	}

	// Validate the key decodes before we write anything, same as LoadPeers.
	raw, err := base64.StdEncoding.DecodeString(entry.RawKey)
	if err != nil {
		return fmt.Errorf("failed to decode pubkey: %v", err)
	}
	if _, err := x509.ParsePKIXPublicKey(raw); err != nil {
		return fmt.Errorf("failed to parse pubkey: %v", err)
	}

	var cfg types.FederationConfig
	if existing, err := os.ReadFile(path); err == nil {
		if err := yaml.Unmarshal(existing, &cfg); err != nil {
			return fmt.Errorf("failed to parse existing federation config at %s: %v", path, err)
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	for _, p := range cfg.Peers {
		if p.ID == entry.ID || p.Name == entry.Name {
			return fmt.Errorf("peer %s (%s) is already in %s", entry.Name, entry.ID, path)
		}
	}

	cfg.Peers = append(cfg.Peers, entry)

	out, err := yaml.Marshal(&cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal federation config: %v", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	return os.WriteFile(path, out, 0644)
}
