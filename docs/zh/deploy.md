# 部署指南

## Docker

```bash
docker build -t execraft:local .
docker run --rm -p 8090:8090 -p 50051:50051 execraft:local
```

## Docker Compose

```bash
docker compose up --build
```

## Kubernetes

```bash
kubectl apply -f k8s/deployment.yaml
kubectl apply -f k8s/service.yaml
```
