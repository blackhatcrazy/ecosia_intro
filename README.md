# ecosia_intro

This app provides a kubernetes service `tree-spotter` in a local minikube cluster that returns
my favorite tree when called like
```bash
curl <clusterIP>/tree -H Host:local.ecosia.org
```

## Prerequisites

To build and deploy this service the following tools must be available in the local environment

- docker
- minikube
- go >= 1.13 (probably also fine with lower versions but untested)
- mv command
- alpine:3.10.2 or docker access to the internet 

## Build and deploy the tree-spotter service

To build this app, change directory to `./scripts` and run

```
go run install.go
```

Per default this will create the tree-spotter app in the namespace `jan`.

## Purge the app

To purge the app run either of the following commands

- `helm del jan-tree-spotter`
- `kubectl delete namespace jan`

