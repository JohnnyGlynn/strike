# Strike
End-to-End Encrypted Messaging service, built on gRPC.

## Implementation status

Implemented:
- Key generation for Client/Server
- Relay server
- Crude stdlib Client REPL
- Client side persistence
- Encrypted messaging

Not implemented:
- Message signing
- Offline sending
- "Account" backup/recovery
- Better key management
- Server to Server communication
- Federation
- Message recovery from friends
- Client TUI
- Install/Deployment procedure
- Message routing based on Load/Geography

## Configuration

Example configuration for Strike can be found in `config/`.

Configuration can be supplied via JSON or Environment variables.
`env.<service>` files and `*Config.json` provide primarily paths to key files required to secure communication for the Strike service.
JSON config must be supplied with `--config=<path to config file>`.

### Keys and Server Certificate
Signing: ED25519 key pair for message origin authenticity via signing messages.
Encryption: Curve25519 key pair used in key exchange to facilitate encryption via shared secret.
Shared secrets: Diffie-Hellman Key Exchange shared secret used between clients for chat encryption.

Key generation can be carried out with the `--keygen` flag for both client and server.
Server key generation will also generate a certificate with its newly generated key pair.

There are Makefile targets for key generation, use `keygen-<client/server>`.

Currently, Strike will generate directories in the Users home directory during key generation, storing it's keys there.
`~/strike-keys` - Client specific keys
`~/strike-server` - Server specific keys + Server's Certificate

Strike is secured with TLS, so your server's certicate file will need to be distributed to users.


## Usage

### Kubernetes

All Kubernetes configuration is present in `config/k8s/`.

`make strike-cluster-start` - Build a local cluster, deploy Server and DB.
`make strike-cluster-stop` - Stop all services and teardown the cluster.

`make bingen` - Builds the client binary, then withing `./build`, creates client1/2, it then executes client1.
`make 2bin` - Runs client2

`make run-client*` - Runs an existing build and db instance of a client within `./build`.

## Commands

`/signup` will enable the client to register a user with the server, followed by logging that User in.

`/login` will enable an existing user access to the strike server, this will then register a status stream on the server, and you should see that your username has logged in. The user status stream will be used to enable Online/Offline status at a later date.

Once the user is logged in:

`/addfriend` shows a list of active users on the server, and prompts to send the selected a friend request.

`/friends` shows the user's friend list, also prompting if they would like to see friend requests they have recieved.

`/invites` will list any pending invites that you have recieved and not responded to. `y` will accept an invite, `n` will decline.

`/chat <username>` enables a chat shell with the given username, retrieving any previous messages in that chat.

## Dependencies
[Docker](https://www.docker.com)/[Podman](https://podman.io)- Container runtimes

[k3d](https://k3d.io) - Lightweight Kubernetes distribution

[ctlptl](https://github.com/tilt-dev/ctlptl) - Cluster management tool

[tilt](https://tilt.dev) - K8s deployment automation

[Protoc](https://grpc.io/docs/protoc-installation/) - for generating Protobuf definition code

