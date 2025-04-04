#!/bin/bash

# export DOCKER_HOST=ssh://root@localhost:53685
# export DOCKER_SOCK=/run/podman/podman.sock

#TODO: create network if not exist
#podman network create strikenw

podman stop strike_server strike_db

# Clean images
podman rm strike_client strike_client1 strike_server strike_db

podman build -t strike_db -f deployment/StrikeDatabase.ContainerFile .

podman run -d --env-file=./config/env.db --name strike_db --network=strikenw -p 5432:5432 localhost/strike_db:latest

podman build -t strike_server -f deployment/StrikeServer.ContainerFile .

podman run -d --env-file=./config/env.server -v ~/.strike-server/:/home/strike-server/ --name strike_server --network=strikenw -p 8080:8080 localhost/strike_server:latest

podman build -t strike_client -f deployment/StrikeClient.ContainerFile .

#only one run container
podman run -it --env-file=./config/env.client -v ~/.strike-keys/:/home/strike-client/ -v ~/.strike-server/strike_server.crt:/home/strike-client/strike_server.crt --name strike_client --network=strikenw localhost/strike_client:latest

#TODO: restart?
# $(CONTAINER_RUNTIME) start -a strike_server

