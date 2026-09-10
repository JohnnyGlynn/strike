# Strike
End-to-End Encrypted Messaging service, built on gRPC.
Strike aims to povide a secure Client-to-Client messaging, with servers acting as relays. Servers can federate with one another so users on different servers can talk; data will otherwise be persisted client side.

## Implementation status

Implemented:
- Key generation for Client/Server
- Relay server
- Crude stdlib Client REPL
- Client side persistence
- Encrypted messaging
- Server to Server federation (see [Federation](#federation))

Planned:
- Offline sending
- "Account" backup/recovery
- Better key management
- Federated user/presence discovery beyond direct `user@domain` addressing
- Message recovery from friends
- Client TUI
- Install/Deployment procedure
- Message routing based on Load/Geography

## Configuration

Example configuration for Strike can be found in `config/`.

Configuration can be supplied via: 
- JSON (`--config=<path/to/config.json>`) 
- Environment variables (`env.<service>` files)

Config files primarily specify key/cert files paths.

### Keys & Certificates

- Signing: ED25519 key pair for message origin authenticity
- Encryption: Curve25519 key pair used for Diffie-Hellman key exchange
- Shared secrets: Derived per-chat for confidentiality

Key generation:

```sh
# client
make keygen-client

#server (also generates Cert)
make keygen-server
```

Currently, Strike will generate directories in the Users home directory during key generation, storing it's keys there.
`~/strike-keys` - Client specific keys
`~/strike-server` - Server specific keys + Server's Certificate

## Usage

After key generation, Strike can be run locally with default config by using the following instructions.

### Local Strike (without k8s)

`make db-build` - Build a Postgres image, and create the relevant tables using `./config/db/init.sql` 
`make db-run` - Run the Postgres container created in the previous step

`make server-run` - Builds and runs a Strike server, using the sample configuration found in `./config/server/serverConfig.json`,

`make bingen` - Builds and runs client binary, making a second client available in `./build/`
`make 2bin` - Runs the Second client binary, allowing for another user to test chat functionality

### Kubernetes

If you would like a more fluid development experience, you can make use of the k3d/Tilt Kubernetes deployment of the Strike Server and DB.
It handles reloading the Server if changes are detected, without you needing to manually intervene.

All Kubernetes configuration is present in `config/k8s/`.

`make strike-cluster-start` - Build a local cluster, deploy Server and DB.
`make strike-cluster-stop` - Stop all services and teardown the cluster.

`make bingen` - Builds the client binary, then withing `./build`, creates client1/2, it then executes client1.
`make 2bin` - Runs client2

`make run-client*` - Runs an existing build and db instance of a client within `./build`.

## Federation

Strike servers can federate with each other so users on different servers can add each other and exchange messages — no shared certificate authority required. Trust between servers is pinned-key, the same model as SSH's `known_hosts`: you record a peer's public key once, and only that exact key is trusted under that identity from then on. There's no signing step and nothing that gets harder as you add more peers — adding peer #100 costs the same as peer #1.

(A shared CA can optionally still be used alongside pinned keys — e.g. the local k3d/Tilt cluster below generates one for its two dev servers — but it's never required.)

### Running your own server

Generate your server's keys and certificate, telling it the address it will actually be reachable at (a public IP, or a dynamic DNS hostname). Without this, TLS connections from clients and other servers will fail — the certificate has to be valid for the address people actually dial:

```sh
make keygen-server SERVER_NAME=<your-name> SAN=<your-address-or-ip>
```

`SERVER_NAME` doubles as your federation domain — it's what goes after the `@` when someone addresses one of your users (e.g. `alice@<your-name>`).

Then follow [Local Strike](#local-strike-without-k8s) above to build the DB and start the server.

You'll also need the ports your server listens on (`8080` for clients, `9090` for federation) reachable from your friend's machine — port-forward them on your router, or use a tunnel (Tailscale, ngrok, etc.) if you'd rather not expose your home connection directly.

### Connecting with a friend

Once you're both running a server, exchange peer entries — a small YAML "peer card" containing your name, address, and public key:

```sh
# Print a shareable entry for your own server
make export-peer SERVER_NAME=<your-name> ADDR=<your-address>:9090 > <your-name>.peer.yaml
```

Send that file to your friend by whatever means (chat, email, USB stick), and have them run:

```sh
make add-peer < <your-name>.peer.yaml
```

Then do the same in the other direction with their exported file. Once both sides have added each other and restarted their server to pick up the change, the two servers can talk. From either client:

```
/addfriend alice@<their-name>
/chat alice@<their-name>
```

A bare username with no `@domain` is always treated as local to your own server.

### Letting a friend log in as a client

Federation is server-to-server — logging a *client* into someone else's server is a separate, simpler step. The client just needs a copy of that server's certificate (`strike_server.crt`, sitting alongside its keys) to trust the connection; it isn't a secret, so just hand over the file. Then point the client at it and at the server's address:

```sh
./strike-client --config=clientConfig.json --server=<their-address>:8080
```

(or set `server_certificate_path` and `server_host` directly in the client's config file).

## Commands

`/signup` will enable the client to register a user with the server, followed by logging that User in.

`/login` will enable an existing user access to the strike server, this will then register a status stream on the server, and you should see that your username has logged in. The user status stream will be used to enable Online/Offline status at a later date.

Once the user is logged in:

`/addfriend` shows a list of active users on the server, and prompts to send the selected a friend request. Given an argument, it looks the user up directly instead — `/addfriend bob` for a local user, or `/addfriend bob@their-server` for a user on a federated server (see [Federation](#federation)).

`/friends` shows the user's friend list, also prompting if they would like to see friend requests they have recieved.

`/invites` will list any pending invites that you have recieved and not responded to. `y` will accept an invite, `n` will decline.

`/chat <username>` or `/chat <username@domain>` enables a chat shell with the given user, retrieving any previous messages in that chat. The domain is only needed to *start* a chat with a federated friend for the first time — once they're a friend, the bare username is enough.

## Dependencies
[Docker](https://www.docker.com)/[Podman](https://podman.io)- Container runtimes

[k3d](https://k3d.io) - Lightweight Kubernetes distribution

[ctlptl](https://github.com/tilt-dev/ctlptl) - Cluster management tool

[tilt](https://tilt.dev) - K8s deployment automation

[Protoc](https://grpc.io/docs/protoc-installation/) - for generating Protobuf definition code

