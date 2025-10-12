k8s_yaml('./deploy/k8s/ns.yaml')

local_resource(
  'strike-namespace',
  'kubectl apply -f ./deploy/k8s/ns.yaml',
  deps=['./deploy/k8s/ns.yaml']
)

local_resource(
  'strike-db1-env',
  'kubectl delete secret strike-db1-env -n strike --ignore-not-found && kubectl create secret generic strike-db1-env --from-env-file=./config/db/env1.db --namespace=strike',
  deps=['./config/db/env1.db'],
  resource_deps=['strike-namespace']
)


local_resource(
  'strike-db1-env',
  'kubectl delete secret strike-db2-env -n strike --ignore-not-found && kubectl create secret generic strike-db2-env --from-env-file=./config/db/env2.db --namespace=strike',
  deps=['./config/db/env2.db'],
  resource_deps=['strike-namespace']
)


local_resource(
  'strike-server1-env',
  'kubectl delete secret strike-server1-env -n strike --ignore-not-found && kubectl create secret generic strike-server1-env --from-env-file=./config/k8s/server1.env --namespace=strike',
  deps=['./config/k8s/server1.env'],
  resource_deps=['strike-namespace']
)


local_resource(
  'strike-server2-env',
  'kubectl delete secret strike-server2-env -n strike --ignore-not-found && kubectl create secret generic strike-server2-env --from-env-file=./config/k8s/server2.env --namespace=strike',
  deps=['./config/k8s/server2.env'],
  resource_deps=['strike-namespace']
)

local_resource(
  'strike-server1-identity',
  'kubectl delete secret strike-server1-identity -n strike --ignore-not-found && kubectl create secret generic strike-server1-identity --from-file=$HOME/.strike-server1 --from-file=./config/server/identity1.json -n strike',
  deps=['$HOME/.strike-server/', './config/server/identity1.json'],
  resource_deps=['strike-namespace']
)

local_resource(
  'strike-server2-identity',
  'kubectl delete secret strike-server2-identity -n strike --ignore-not-found && kubectl create secret generic strike-server2-identity --from-file=$HOME/.strike-server2 --from-file=./config/server/identity2.json -n strike',
  deps=['$HOME/.strike-server2/', './config/server/identity2.json'],
  resource_deps=['strike-namespace']
)


local_resource(
  'strike-federation',
  'kubectl delete secret strike-federation -n strike --ignore-not-found && kubectl create secret generic strike-federation --from-file=./config/server -n strike',
  deps=['./config/server/federation.yaml'],
  resource_deps=['strike-namespace']
)

k8s_yaml([
  './deploy/k8s/db.yaml',
  './deploy/k8s/db-svc.yaml',
  './deploy/k8s/db2.yaml',
  './deploy/k8s/db2-svc.yaml',
  './deploy/k8s/server.yaml',
  './deploy/k8s/server-svc.yaml',
  './deploy/k8s/server2.yaml',
  './deploy/k8s/server2-svc.yaml',
])

docker_build(
  'strike-db',
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

k8s_resource(
  'strike-db-1',
  port_forwards=5432,
  resource_deps=['strike-db-env']
)

k8s_resource(
  'strike-server-1',
  port_forwards=8080,
  resource_deps=[
      'strike-server-env',
      'strike-server-identity',
      'strike-federation',
      'strike-db-1'
  ]
)

k8s_resource(
  'strike-db-2',
  port_forwards=5432,
  resource_deps=['strike-db-env']
)

k8s_resource(
  'strike-server-2',
  port_forwards=8080,
  resource_deps=[
      'strike-server-env',
      'strike-server-identity',
      'strike-federation',
      'strike-db-2'
  ]
)


