k8s_yaml('./deployment/k8s/ns.yaml')

local_resource(
  'strike-namespace',
  'kubectl apply -f ./deployment/k8s/ns.yaml',
  deps=['./deployment/k8s/ns.yaml']
)

local_resource(
  'strike-db-env',
  'kubectl delete secret strike-db-env -n strike --ignore-not-found && kubectl create secret generic strike-db-env --from-env-file=./config/env.db --namespace=strike',
  deps=['./config/env.db', 'strike-namespace']
)

local_resource(
  'strike-server-env',
  'kubectl delete secret strike-server-env -n strike --ignore-not-found && kubectl create secret generic strike-server-env --from-env-file=./config/k8s/server.env --namespace=strike',
  deps=['./config/k8s/server.env', 'strike-namespace']
)

local_resource(
  'strike-client-env',
  'kubectl delete secret strike-client-env -n strike --ignore-not-found && kubectl create secret generic strike-client-env --from-env-file=./config/k8s/client.env --namespace=strike',
  deps=['./config/k8s/client.env', 'strike-namespace']
)


local_resource(
  'strike-server-identity',
  'kubectl delete secret strike-server-identity -n strike --ignore-not-found && kubectl create secret generic strike-server-identity --from-file=$HOME/.strike-server -n strike',
  deps=['$HOME/.strike-server/', 'strike-namespace']
)

local_resource(
  'strike-keys',
  'kubectl delete secret strike-keys -n strike --ignore-not-found && kubectl create secret generic strike-keys --from-file=$HOME/.strike-keys -n strike',
  deps=['$HOME/.strike-keys/', 'strike-namespace']
)


k8s_yaml([
  './deployment/k8s/db.yaml',
  './deployment/k8s/db-svc.yaml',
  './deployment/k8s/server.yaml',
  './deployment/k8s/server-svc.yaml',
  './deployment/k8s/client.yaml',
])

docker_build('strike_db', './', dockerfile='deployment/StrikeDatabase.ContainerFile')
docker_build('strike_server', './', dockerfile='deployment/StrikeServer.ContainerFile')
docker_build('strike_client', './', dockerfile='deployment/StrikeClient.ContainerFile')

k8s_resource('strike-db', port_forwards=5432, resource_deps=['strike-db-env'])
k8s_resource('strike-server', port_forwards=8080, resource_deps=['strike-server-env'])

