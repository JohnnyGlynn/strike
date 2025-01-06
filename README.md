# Strike

End-to-End Encrypted Messaging service using gRPC

## Running Strike Locally

The following instructions may reference `podman` as the container runtime, but `docker` is interchangeable here.

The makefile will account for whether you are using Docker or Podman (target: check-runtime)

## Keys

Signing: ED25519 key pair is used for signing messages to ensure that the sender of a message is authentic.
Encryption: Curve25519 key pair is used for message encryption Key-Exchange/Encryption/Key derivation

Keys that are yet to be implemented:
- Ephemeral Key Pair - Session keys
- Symmetric Key Pair - Message Encryption

Configuration references a key path for `~/.strike-keys/`, this is where keys will be placed by default if you make use of the strike client binary `--keygen` flag. This generates 2 sets of long term keys (ED25519 signing keys & Curve25519 encryption keys)

This will generate you a private and public key that can be then used with the client.

## Certificate
To use Strike with TLS enabled you will need to generate a Certificate for your server using its private key and distribute that to your users.
Using the `--keygen` flag with the server binary (`go build cmd/strike-server/main.go && ./cmd/strike-server/main.go --keygen`) will automatically generate a certificate for you.
Like the server keys these can be found at `~/.strike-server`.

### Configuration
The Client currently supports config from either a JSON file or ENV vars.

If you wish to run the Client in a container use make target `client-container-run`, but ensure that you provide config to the container correctly, ENV vars recommended:

    SERVER_HOST=strike_server:8080
    USERNAME=<Username of your choice>
    PRIVATE_SIGNING_KEY_PATH=/home/.strike-keys/strike_signing.pem
    PUBLIC_SIGNING_KEY_PATH=/home/.strike-keys/strike_public_signing.pem
    PRIVATE_ENCRYPTION_KEY_PATH=/home/.strike-keys/strike_encryption.pem
    PUBLIC_ENCRYPTION_KEY_PATH=/home/.strike-keys/strike_public_encryption.pem
    CLIENT_SERVER_CERT_PATH=/home/strike-client/strike_server.crt

The Server container also requires Env vars to specify paths to keys and its certificate.

    SERVER_NAME=endpoint0
    PRIVATE_SIGNING_KEY_PATH=/home/strike-server/strike_server.pem
    PUBLIC_SIGNING_KEY_PATH=/home/strike-server/strike_server_public.pem
    SERVER_CERT_PATH=/home/strike-server/strike_server.crt

These Env vars are loaded into the containers by their repective Makefile targets and currently only work for a local deployment,
but should you wish to deploy Strike another way, `env.server` & `env.client` can be found in the config directory.

### Container runtime network
Create a network for strike using the following:
```bash
podman network create strikenw
```
This will facilitate our container's communicating

<!-- TODO: [k3d](https://k3d.io/stable/) + [tilt](https://tilt.dev/) as a means for Docker users or Implementing [Podman pods](https://docs.podman.io/en/v5.2.5/markdown/podman-pod-create.1.html) directly.

Either way the K8s manifests will be rolled once and used as needed. -->

### Postgres

Build DB container
```bash
make db-container-build
```
Run DB container
```bash
make db-container-run
```

### Server
Build Server container
```bash
make server-container-build
```
Run Server container
```bash
make server-container-run
```

### Client
Build client container
```bash
make client-container-build
```
Run client container
```bash
make client-container-run
```

Build client binary
```bash
make client-binary-build
```

Run client binary
```bash
make client-binary-run
```


## Dependencies
[Protoc](https://grpc.io/docs/protoc-installation/)
[]()
