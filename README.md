# Strike

The following instructions may reference `podman` as the container runtime, but `docker` is interchangeable here.

The makefile will account for whether you are using Docker or Podman (target: check-runtime)

### Container runtime network

Create a network for strike using the following:
```bash
podman network create strikenw
```

For now will facilitate Networking between the Server and the Database, but will change when Strike is migrated to an orchestrated architecture

TODO: [k3d](https://k3d.io/stable/) + [tilt](https://tilt.dev/) as a means for Docker users or Implementing [Podman pods](https://docs.podman.io/en/v5.2.5/markdown/podman-pod-create.1.html) directly.

Either way the K8s manifests will be rolled once and used as needed.

## Running Strike Client/Server

### Client
```bash
make run-strike-client
```

### Server
```bash
make run-server-container
```

### Postgres
```bash
make run-db-container
```

## Dependencies
[Protoc](https://grpc.io/docs/protoc-installation/)
[]()
