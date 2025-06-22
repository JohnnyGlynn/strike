#Strike Server

FROM golang:tip-alpine AS builder

RUN apk add --no-cache bash curl

WORKDIR /go/strike

COPY go.mod go.sum ./
RUN go mod download

COPY ./ ./
RUN rm -rf ./internal/client ./deploy ./config ./cmd/strike-client

RUN go build -o strike.bin ./cmd/strike-server/main.go 

FROM scratch

COPY --from=builder /go/strike/strike.bin /strike
EXPOSE 8080

CMD ["/strike"]
