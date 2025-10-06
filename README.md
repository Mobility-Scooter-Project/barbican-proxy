# Barbican Proxy
This repo contains a simple Go API for proxying request to OpenStack Barbican

**Note:** This API server does not contain any security measures and is designed to be run cluster-side only.

## Getting Started
Make sure that you have the following tools installed:
- [Go 1.24](https://go.dev/doc/install)
- [Docker & Docker Compose](https://docs.docker.com/compose/install/)

### Building
To build the app, use the standard go build command:
```sh
go build -o bin/barbican-proxy
```

To build the docker image, run the following command:
```sh
docker buildx build . -t barbican-proxy:latest
```

### Running
To run the proxy for development, use the standard go run command:
```sh
go run .
```

To run the proxy for production, execute the binary after building it:
```sh
./bin/barbican-proxy
```
*Instructions may be different for Windows users*

To run the docker image locally, use the following command:
```sh
docker run --network host -it barbican-proxy:latest
```

#### Docker Compose
This repo uses docker compose to emulate Valkey locally. To run it, use the following command:
```
docker compose up -d
```
**Note**: Be sure to close any running KV containers!