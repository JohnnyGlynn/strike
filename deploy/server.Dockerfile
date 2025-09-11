#Strike Server

FROM golang:1.23-alpine AS builder
WORKDIR /go/strike
# RUN apk add --no-cache bash curl
RUN apk add --no-cache ca-certificates


COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -o strike.bin ./cmd/strike-server

FROM scratch
#tls COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /go/strike/strike.bin /strike

EXPOSE 8080
CMD ["/strike"]
