package server

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/JohnnyGlynn/strike/internal/config"
	"github.com/JohnnyGlynn/strike/internal/server/types"

	"google.golang.org/grpc"
)

// freePort asks the kernel for an unused port, then releases it. Racy in
// principle, fine in a test — nothing else here is binding ports.
func freePort(t *testing.T) int {
	t.Helper()

	lis, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("failed to find a free port: %v", err)
	}
	defer func() { _ = lis.Close() }()

	return lis.Addr().(*net.TCPAddr).Port
}

// testBootstrap is the minimum Start() needs: two gRPC servers and a
// StrikeServer with an empty PeerManager, so ConnectAll is a no-op.
func testBootstrap(clientPort, federationPort int) *Bootstrap {
	return &Bootstrap{
		Cfg: config.ServerConfig{
			ClientPort:     clientPort,
			FederationPort: federationPort,
		},
		Strike:     &StrikeServer{PeerMgr: NewPeerManager([]types.PeerConfig{})},
		grpcStrike: grpc.NewServer(),
		grpcFed:    grpc.NewServer(),
	}
}

func dialable(port int) bool {
	conn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func TestStartBindsConfiguredPorts(t *testing.T) {
	clientPort, federationPort := freePort(t), freePort(t)

	b := testBootstrap(clientPort, federationPort)
	if err := b.Start(context.Background()); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() {
		b.grpcStrike.Stop()
		b.grpcFed.Stop()
	}()

	if !dialable(clientPort) {
		t.Errorf("client port %d is not accepting connections", clientPort)
	}

	if !dialable(federationPort) {
		t.Errorf("federation port %d is not accepting connections", federationPort)
	}
}

// Stop used to print "shutdown strike server" without stopping anything, so
// the listeners stayed bound and only process exit cleaned them up.
func TestStopReleasesPorts(t *testing.T) {
	clientPort, federationPort := freePort(t), freePort(t)

	b := testBootstrap(clientPort, federationPort)
	if err := b.Start(context.Background()); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	b.Stop()

	if dialable(clientPort) {
		t.Errorf("client port %d still bound after Stop", clientPort)
	}

	if dialable(federationPort) {
		t.Errorf("federation port %d still bound after Stop", federationPort)
	}

	// Stop must leave the ports free for an immediate restart.
	restarted := testBootstrap(clientPort, federationPort)
	if err := restarted.Start(context.Background()); err != nil {
		t.Fatalf("could not rebind after Stop: %v", err)
	}
	restarted.Stop()
}

// A client that opens a socket and says nothing blocks grpc's GracefulStop
// *and* its Stop until the connection timeout (120s by default) expires. Stop
// has to give up on draining and return anyway, having freed the ports.
func TestStopIsBoundedByShutdownGrace(t *testing.T) {
	if testing.Short() {
		t.Skipf("waits out the full %s grace period", ShutdownGrace)
	}

	clientPort, federationPort := freePort(t), freePort(t)

	b := testBootstrap(clientPort, federationPort)
	if err := b.Start(context.Background()); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	mute, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", clientPort))
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer func() { _ = mute.Close() }()

	done := make(chan struct{})
	start := time.Now()
	go func() {
		b.Stop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(ShutdownGrace + 10*time.Second):
		t.Fatalf("Stop hung past the %s grace period with a silent connection open", ShutdownGrace)
	}

	if elapsed := time.Since(start); elapsed < ShutdownGrace {
		t.Errorf("Stop returned after %s — expected it to wait out the %s grace period", elapsed, ShutdownGrace)
	}

	if dialable(clientPort) {
		t.Errorf("client port %d still bound after Stop gave up", clientPort)
	}

	if dialable(federationPort) {
		t.Errorf("federation port %d still bound after Stop gave up", federationPort)
	}
}

func TestStopIsSafeWithNothingStarted(t *testing.T) {
	// Stop runs from main's shutdown path, which can be reached before every
	// component is up.
	(&Bootstrap{}).Stop()
}

// The bug this replaces: Start() listened inside the goroutines and discarded
// the error, so a taken port produced a running server that was deaf on it.
func TestStartFailsOnPortClash(t *testing.T) {
	cases := map[string]struct{ takeClient bool }{
		"client-port-taken":     {takeClient: true},
		"federation-port-taken": {takeClient: false},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			clientPort, federationPort := freePort(t), freePort(t)

			taken := federationPort
			if tc.takeClient {
				taken = clientPort
			}

			squatter, err := net.Listen("tcp", fmt.Sprintf(":%d", taken))
			if err != nil {
				t.Fatalf("failed to occupy port %d: %v", taken, err)
			}
			defer func() { _ = squatter.Close() }()

			b := testBootstrap(clientPort, federationPort)
			if err := b.Start(context.Background()); err == nil {
				b.grpcStrike.Stop()
				b.grpcFed.Stop()
				t.Fatalf("Start succeeded with port %d already taken", taken)
			}

			// A failed Start must not leave the port it did get bound.
			if !tc.takeClient && dialable(clientPort) {
				t.Errorf("client port %d left bound after a failed Start", clientPort)
			}
		})
	}
}
