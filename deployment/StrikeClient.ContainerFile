# Copyright - Watchful Inc. 2024

#Strike Client

# Use an official Go image based on Alpine
FROM golang:alpine AS builder

WORKDIR /go/strike

# Copy go dependency files
COPY go.mod go.sum ./

# Use cached dependencies if mod/sum not changed
RUN go mod download

# Copy everything and chop out whats not needed
COPY ./ ./

# rm -rf rather than implementing multipel .dockerignores
RUN rm -rf ./internal/server ./deployment ./config ./cmd/strike-server

RUN go build -o /go/strike-client cmd/strike-client/client.go 

# minimal base image
FROM alpine:latest
WORKDIR /root/
COPY --from=builder /go/strike-client .

CMD ["./strike-client"]
