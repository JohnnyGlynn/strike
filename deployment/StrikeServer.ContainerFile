# Copyright - Watchful Inc. 2024

#Strike Server

# Use an official Go image based on Alpine
FROM golang:alpine

# Install base packages
RUN apk add --no-cache bash curl

# Install OpenAPI Generator
# RUN go install github.com/deepmap/oapi-codegen/v2/cmd/oapi-codegen@latest
# RUN oapi-codegen -version

# Install alpine respositories
RUN echo 'http://dl-cdn.alpinelinux.org/alpine/v3.6/main' >> /etc/apk/repositories
RUN echo 'http://dl-cdn.alpinelinux.org/alpine/v3.6/community' >> /etc/apk/repositories
RUN apk update

EXPOSE 8080

WORKDIR /go/strike

COPY go.mod go.sum ./
RUN go mod download

# TODO: dont copy everything
COPY ./ ./

RUN go mod tidy

RUN CGO_ENABLED=0 go build -o /go/strike.bin cmd/strike-server/main.go 

CMD ["/go/strike.bin"]
