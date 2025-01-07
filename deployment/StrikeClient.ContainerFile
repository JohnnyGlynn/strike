# Copyright - Watchful Inc. 2024

#Strike Client

# Use an official Go image based on Alpine
FROM golang:alpine

WORKDIR /go/strike

# Copy everything and chop out whats not needed
COPY ./ ./

# rm -rf rather than implementing multipel .dockerignores
RUN rm -rf ./internal/db ./internal/server ./deployment ./config ./cmd/strike-server

RUN go mod download 

RUN CGO_ENABLED=0 go build -o /go/strike-client cmd/strike-client/main.go 

CMD ["/go/strike-client"]
