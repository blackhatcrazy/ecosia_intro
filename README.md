# ecosia_intro

This app provides a kubernetes service `tree-spotter` in a local minikube cluster that returns
my favorite tree when called like
```bash
curl ${MINIKUBE_IP}/tree -H Host:local.ecosia.org
curl localhost:7000/tree -H Host:local.ecosia.org
```

## Prerequisites

To build and deploy this service the following tools must be available in the local environment

- docker >= 19.03.2 (probably also fine with lower versions but untested)
- minikube >= 1.3.1 (probably also fine with lower versions but untested)
- go >= 1.13 (probably also fine with lower versions but untested)
- mv command
- alpine:3.10.2 or docker access to the internet 

## Build and deploy the tree-spotter service

To build this app, the following environment variables must be set

```bash
export KUBECONFIG="<kubeconfig-path for the minkube>"
export DOCKER_TLS_VERIFY="<is set by eval $(minikube docker-env)>"  # on windows different
export DOCKER_HOST="<is set by eval $(minikube docker-env)>"  # on windows different
export DOCKER_CERT_PATH="<is set by eval $(minikube docker-env)>"  # on windows different
export MINIKUBE_IP=$(minikube ip) # on windows different
```

Change directory to `./scripts` and run

```
go run install.go
```

If you prefer to execute the build binary run in the `./scripts` folder

- mac: `./install.darwin.amd64`
- linux: `./install`
- windows: `install.exe`

Per default this will create the tree-spotter app in the namespace `jan`.

It can be viewed from the `helm3` cli by calling `./binaries/helm3 list -n jan` (change the helm binary according to operating system).


## Accessing the homepage

The running homepage can now be reached under 

`curl ${MINIKUBE_IP}/tree -H Host:local.ecosia.org`

To make the homepage also available under `localhost (127.0.0.1)` do the following:

- open a terminal
- run `kubectl port-forward svc/tree-spotter 7000:8080 -n jan` (this forwards the localhost port 7000 to the tree-spotter service listening on 8080 in the minikube)
- leave the terminal open and open a new one

Now the homepage can also be reached under

`curl localhost:7000/tree -H Host:local.ecosia.org`

## Delete the app & cleanup

To purge the app run either of the following commands

- `helm del jan-tree-spotter --namespace jan`
- `kubectl delete namespace jan`

Deleting the docker images (only after purging the app)

`docker rmi -f $(docker images | grep 'tree-spotter')`

Remove dangling docker images

`docker rmi $(docker images -f "dangling=true" -q)`


