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

In the root directory
```bash
podman build -t strike_server -f PodmanFile
podman run -p 8080:8080  localhost/strike_server:latest
```


