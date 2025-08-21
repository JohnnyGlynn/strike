# Strike
End-to-End Encrypted Messaging service, built on gRPC.
Strike aims to povide a secure Client-to-Client messaging, with servers acting as relays. With the exception of future User federation, data will be persisted client side.

## Implementation status

Implemented:
- Key generation for Client/Server
- Relay server
- Crude stdlib Client REPL
- Client side persistence
- Encrypted messaging

Planned:
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

