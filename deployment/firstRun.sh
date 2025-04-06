#!/bin/bash

set -e

CLUSTER_NAME="StrikeMessagingService"

echo "Existing cluster teardown."
k3d cluster delete $CLUSTER_NAME || true

k3d cluster create $CLUSTER_NAME --agents 1 -p "8080:80@loadbalancer" -p "5432:5432@loadbalancer"

podman build -t strike_db -f deployment/StrikeDatabase.ContainerFile .
podman build -t strike_server -f deployment/StrikeServer.ContainerFile .
podman build -t strike_server -f deployment/StrikeServer.ContainerFile .

k3d image import strike_db strike_server strike_client -c $CLUSTER_NAME

#Kubectl we can replace with helm
kubectl apply -f k8s/configs/
kubectl apply -f k8s/db-deployment.yaml
kubectl apply -f k8s/server-deployment.yaml
kubectl apply -f k8s/client-pod.yaml
kubectl wait --for=condition=ready pod -l app=strike-db --timeout=60s
kubectl wait --for=condition=ready pod -l app=strike-server --timeout=60s

echo "Cluster setup complete."
