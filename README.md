# Strike

## dependencies
[Protoc](https://grpc.io/docs/protoc-installation/)
[]()

## Running Strike Client/Server

### Client

In `cmd/strike-client`
```bash
go build client.go && ./client
```

### Server

The following instructions will reference `podman` as the method of running containers, but `docker` is interchangeable here.

The following is for testing locally, bypassing network isolation with `--net=host`.

In the root directory
```bash
podman build -t strike_server -f StrikeServer.ContainerFile
podman run --net=host  localhost/strike_server:latest
```


### Postgres

In the root directory
```bash
podman build -t strike_db -f StrikeDatabase.ContainerFile
podman run -p 5432:5432  localhost/strike_db:latest
```


