k8s_yaml('./deploy/k8s/ns.yaml')

local_resource(
  'strike-namespace',
  'kubectl apply -f ./deploy/k8s/ns.yaml',
  deps=['./deploy/k8s/ns.yaml']
)

local_resource(
  'strike-db-env',
  'kubectl delete secret strike-db-env -n strike --ignore-not-found && kubectl create secret generic strike-db-env --from-env-file=./config/db/env.db --namespace=strike',
  deps=['./config/env.db', 'strike-namespace']
)

local_resource(
  'strike-server-env',
  'kubectl delete secret strike-server-env -n strike --ignore-not-found && kubectl create secret generic strike-server-env --from-env-file=./config/k8s/server.env --namespace=strike',
  deps=['./config/k8s/server.env', 'strike-namespace']
)

local_resource(
  'strike-server-identity',
  'kubectl delete secret strike-server-identity -n strike --ignore-not-found && kubectl create secret generic strike-server-identity --from-file=$HOME/.strike-server -n strike',
  deps=['$HOME/.strike-server/', 'strike-namespace']
)

k8s_yaml([
  './deploy/k8s/db.yaml',
  './deploy/k8s/db-svc.yaml',
  './deploy/k8s/server.yaml',
  './deploy/k8s/server-svc.yaml',
])

docker_build(
  'strike_db',
  './',
  dockerfile='deploy/db.Dockerfile',
  ignore=['build', 'cmd', 'internal']
)
docker_build(
  'strike_server',
  './',
  dockerfile='deploy/server.Dockerfile',
  ignore=['build', 'cmd/strike-client', 'internal/client']
)

k8s_resource('strike-db', port_forwards=5432, resource_deps=['strike-db-env'])
k8s_resource('strike-server', port_forwards=8080, resource_deps=['strike-server-env', 'strike-db'])

