# Strike
Distributed End-to-End Encrypted Messaging service, built on gRPC.

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

All Kubernetes configuration is present in `config/k8s/`.

## Usage

Currently there are two methods of running Strike locally:
Containers locally on the host machine.
K8s deployment of the Server and Database, accessable to client containers on the host machine.

### Containers

Using the provided make targets and service (db/server/client):

`make *-build` - Build the service.
`make *-run` - Run the latest image on the machine, name the container, and provide the relevant config.
`make *-start` - Restart an existing container.

`make another-client-run` will run an additional client with the same keys to provide a secondary user to chat with.

### Kubernetes

`make strike-cluster-start` - Build a local cluster, deploy Server and DB.
`make strike-cluster-stop` - Stop all services and teardown the cluster.

`make client-run` - Use this to create a client and connect it to the cluster via Config.

## Commands

`/signup` will enable the client to register a user with the server, followed by logging that User in.

`/login` will enable an existing user access to the strike server, this will then register a status stream on the server, and you should see that your username has logged in. The user status stream will be used to enable Online/Offline status at a later date.

Once the user is logged in:

`/msgshell` will enable an interactive messaging shell, from here you will be able to send messages to yourself or other clients message streams.

This however, requires an active Chat.

`/beginchat` will allow you to send a chat invite to another user, you will be prompted for a username, and if they are online an invite will be sent.

`/invites` will list any pending invites that you have recieved and not responded to. `y` will accept an invite, `n` will decline.

`/chats` will list chats that you have joined via Invite, and will allow you to set one as active. This active chat will allow you to send messages.

Inputting `<target username>:<message>` will deliver a message to the targetted user (i.e. `client0:Hello World!`)

## Dependencies
[Docker](https://www.docker.com)/[Podman](https://podman.io)- Container runtimes

[k3d](https://k3d.io) - Lightweight Kubernetes distribution

[ctlptl](https://github.com/tilt-dev/ctlptl) - Cluster management tool

[tilt](https://tilt.dev) - K8s deployment automation

[Protoc](https://grpc.io/docs/protoc-installation/) - for generating Protobuf definition code

