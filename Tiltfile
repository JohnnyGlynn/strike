docker_build('strike_db, '.', dockerfile='./deployment/StrikeDatabase.ContainerFile')
docker_build('strike_server, '.', dockerfile='./deployment/StrikeServer.ContainerFile')
docker_build('strike_client, '.', dockerfile='./deployment/StrikeClient.ContainerFile')

k8s_yaml('./deployment/k8s/db.yaml')
k8s_resource('strike_db', port_forwards=5432)

k8s_yaml('./deployment/k8s/server.yaml')
k8s_resource('strike_server', port_forwards=8080)

k8s_yaml('./deployment/k8s/client.yaml')

