package server

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/JohnnyGlynn/strike/internal/config"
	"github.com/JohnnyGlynn/strike/internal/keys"
	"github.com/JohnnyGlynn/strike/internal/server/types"
	fedpb "github.com/JohnnyGlynn/strike/msgdef/federation"
	pb "github.com/JohnnyGlynn/strike/msgdef/message"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type Bootstrap struct {
	Cfg        config.ServerConfig
	DB         *pgxpool.Pool
	Statements *ServerDB

	Strike       *StrikeServer
	Orchestrator *FederationOrchestrator

	grpcStrike *grpc.Server
	grpcFed    *grpc.Server

	fedTLS *tls.Config
}

func InitBootstrap(cfg config.ServerConfig) *Bootstrap {
	return &Bootstrap{Cfg: cfg}
}

func (b *Bootstrap) InitDb(ctx context.Context) error {
	pgConfig, err := pgxpool.ParseConfig(b.Cfg.DBConnectionString)
	if err != nil {
		return err
	}

	b.DB, err = dbWithRetry(ctx, pgConfig)
	if err != nil {
		return err
	}

	b.Statements, err = InitStatements(ctx, b.DB)
	return err
}

func dbWithRetry(ctx context.Context, pgConfig *pgxpool.Config) (*pgxpool.Pool, error) {
	var pool *pgxpool.Pool
	var err error

	backoff := time.Second

	for i := 0; i < 30; i++ {
		pool, err = pgxpool.NewWithConfig(ctx, pgConfig)
		if err == nil {
			pingErr := pool.Ping(ctx)
			if pingErr == nil {
				return pool, nil
			}
			err = pingErr
		}

		fmt.Printf("DB not ready (%v). Retrying: %s...", err, backoff)

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}

		if backoff < 10*time.Second {
			backoff *= 2
		}
	}

	return nil, fmt.Errorf("db unavailable: %w", err)
}

func (b *Bootstrap) InitStrikeServer(creds credentials.TransportCredentials, peers []types.PeerConfig) error {
	key, err := keys.GetKeyFromPath(b.Cfg.SigningPublicKeyPath)
	if err != nil {
		return err
	}

	id := DeriveServerID(key)

	b.Strike = &StrikeServer{
		Name:           b.Cfg.Name,
		ID:             uuid.MustParse(id),
		DBpool:         b.DB,
		PStatements:    b.Statements,
		PeerMgr:        NewPeerManager(peers),
		Pending:        make(map[uuid.UUID]*types.PendingMsg),
		RemotePresence: make(map[uuid.UUID]string),
	}
	b.grpcStrike = grpc.NewServer(
		grpc.Creds(creds),
	)

	pb.RegisterStrikeServer(b.grpcStrike, b.Strike)
	return nil
}

func (b *Bootstrap) InitFederation() error {
	var err error

	b.fedTLS, err = LoadFederationTLSConfig(
		b.Cfg.CertificatePath,
		b.Cfg.SigningPrivateKeyPath,
		b.Cfg.FederationCAPath,
		b.Strike.PeerMgr.Peers(),
	)
	if err != nil {
		return err
	}

	b.Orchestrator = NewFederationOrchestrator(b.Strike)

	b.grpcFed = grpc.NewServer(
		grpc.Creds(credentials.NewTLS(b.fedTLS)),
	)

	fedpb.RegisterFederationServer(b.grpcFed, b.Orchestrator)
	return nil
}

// LoadFederationTLSConfig builds the mTLS config used both to dial peers and
// to accept peer connections on the federation port.
//
// Trust is pinned-key by default: a peer's certificate is accepted if its
// public key matches one of the entries in the known peers list — the same
// known_hosts model as federation.yaml itself (see PeerConfig.PubKey and
// `--export-peer`/`--add-peer`). Adding a peer means recording their key
// once; there is no CA, no signing step, and nothing to bottleneck as the
// peer list grows.
//
// If caFile is non-empty, a peer certificate that chains to it is *also*
// accepted, alongside pinned-key trust rather than instead of it — for
// deployments (e.g. the local k3d/Tilt cluster) that want one shared CA
// across a group of servers they control.
func LoadFederationTLSConfig(certFile, keyFile, caFile string, peers []types.PeerConfig) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}

	var caPool *x509.CertPool
	if caFile != "" {
		caPEM, err := os.ReadFile(caFile)
		if err != nil {
			return nil, err
		}

		caPool = x509.NewCertPool()
		if !caPool.AppendCertsFromPEM(caPEM) {
			return nil, fmt.Errorf("invalid CA pem")
		}
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		// A certificate must still be presented, but we verify it ourselves
		// below (pinned key, and CA chain if configured) rather than via Go's
		// default chain verification, which would require every peer to
		// share one CA.
		ClientAuth:            tls.RequireAnyClientCert,
		InsecureSkipVerify:    true,
		VerifyPeerCertificate: verifyKnownPeer(caPool, peers),
		MinVersion:            tls.VersionTLS13,
	}, nil
}

// verifyKnownPeer accepts a peer certificate if its public key matches a
// pinned peer, or (when caPool is configured) if it chains to that CA.
func verifyKnownPeer(caPool *x509.CertPool, peers []types.PeerConfig) func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
	return func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
		if len(rawCerts) == 0 {
			return fmt.Errorf("federation: peer presented no certificate")
		}

		leaf, err := x509.ParseCertificate(rawCerts[0])
		if err != nil {
			return fmt.Errorf("federation: failed to parse peer certificate: %v", err)
		}

		leafPub, ok := leaf.PublicKey.(ed25519.PublicKey)
		if !ok {
			return fmt.Errorf("federation: unsupported peer key type %T", leaf.PublicKey)
		}

		for _, p := range peers {
			if len(p.PubKey) > 0 && leafPub.Equal(p.PubKey) {
				return nil
			}
		}

		if caPool != nil {
			intermediates := x509.NewCertPool()
			for _, raw := range rawCerts[1:] {
				if c, err := x509.ParseCertificate(raw); err == nil {
					intermediates.AddCert(c)
				}
			}

			if _, err := leaf.Verify(x509.VerifyOptions{
				Roots:         caPool,
				Intermediates: intermediates,
				KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
			}); err == nil {
				return nil
			}
		}

		return fmt.Errorf("federation: peer's public key is not a known peer (check federation.yaml) and its certificate is not signed by a trusted CA")
	}
}

func (b *Bootstrap) InitFederationTLS() (*tls.Config, error) {
	tlsConf, err := LoadFederationTLSConfig(
		b.Cfg.CertificatePath,       // server cert
		b.Cfg.SigningPrivateKeyPath, // server key
		b.Cfg.FederationCAPath,      // optional CA that signs peer certs
		b.Strike.PeerMgr.Peers(),
	)
	if err != nil {
		return nil, err
	}

	fmt.Println("Loaded federation mTLS config")
	return tlsConf, nil
}

func (b *Bootstrap) Start(ctx context.Context) error {
	if b.grpcStrike == nil || b.grpcFed == nil || b.Strike == nil {
		return fmt.Errorf("bootstrap not initialized")
	}

	// Bind before backgrounding either server. Listening inside the goroutines
	// meant a port clash produced a process that looked healthy and was deaf.
	clientAddr := fmt.Sprintf(":%d", b.Cfg.ClientPort)
	clientLis, err := net.Listen("tcp", clientAddr)
	if err != nil {
		return fmt.Errorf("failed to listen for clients on %s: %w", clientAddr, err)
	}

	fedAddr := fmt.Sprintf(":%d", b.Cfg.FederationPort)
	fedLis, err := net.Listen("tcp", fedAddr)
	if err != nil {
		_ = clientLis.Close()
		return fmt.Errorf("failed to listen for federation on %s: %w", fedAddr, err)
	}

	fmt.Printf("Strike listening for clients on %s, federation on %s\n", clientAddr, fedAddr)

	go func() {
		if err := b.grpcStrike.Serve(clientLis); err != nil {
			fmt.Fprintf(os.Stderr, "strike server stopped serving %s: %v\n", clientAddr, err)
		}
	}()

	go func() {
		if err := b.grpcFed.Serve(fedLis); err != nil {
			fmt.Fprintf(os.Stderr, "federation server stopped serving %s: %v\n", fedAddr, err)
		}
	}()

	go func() {
		b.Strike.PeerMgr.ConnectAll(
			ctx,
			b.fedTLS,
			b.Strike.ID.String(),
			b.Strike.Name,
		)
	}()

	return nil
}

// ShutdownGrace bounds how long Stop waits for the gRPC servers to drain. A
// bound is not optional: clients hold PayloadStream open for as long as
// they're connected and GracefulStop waits on open streams, so an unbounded
// wait would hang until every client happened to disconnect. Kept well under
// the 30s Kubernetes allows by default before it sends SIGKILL.
const ShutdownGrace = 10 * time.Second

// Stop shuts down both gRPC servers, drops outbound federation connections and
// closes the DB pool. It takes no context deliberately — by the time it runs
// the caller's context is already cancelled, so it can't serve as the deadline.
func (b *Bootstrap) Stop() {
	fmt.Println("Strike shutting down")

	servers := []struct {
		name string
		srv  *grpc.Server
	}{
		{"strike server", b.grpcStrike},
		{"federation server", b.grpcFed},
	}

	drained := make(chan struct{})
	go func() {
		var wg sync.WaitGroup

		for _, s := range servers {
			if s.srv == nil {
				continue
			}

			wg.Add(1)
			go func() {
				defer wg.Done()
				s.srv.GracefulStop()
				fmt.Printf("%s drained\n", s.name)
			}()
		}

		wg.Wait()
		close(drained)
	}()

	select {
	case <-drained:
	case <-time.After(ShutdownGrace):
		// Both GracefulStop and Stop close their listeners before they wait,
		// so the ports are already free by now. What's left is a connection
		// that opened a socket and never finished the HTTP/2 handshake, which
		// blocks both of them identically until grpc's connection timeout
		// expires — Stop can't be rescued by a second Stop. Fire it anyway to
		// kill established transports, but don't wait on it; the process is
		// exiting and the kernel reclaims the rest.
		fmt.Printf("servers did not drain in %s, closing transports and continuing\n", ShutdownGrace)
		for _, s := range servers {
			if s.srv != nil {
				go s.srv.Stop()
			}
		}
	}

	if b.Strike != nil && b.Strike.PeerMgr != nil {
		b.Strike.PeerMgr.CloseAll()
	}

	if b.DB != nil {
		b.DB.Close()
	}

	fmt.Println("Shutdown complete")
}
