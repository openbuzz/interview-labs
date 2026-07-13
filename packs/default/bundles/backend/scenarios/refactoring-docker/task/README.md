# Go Web App

A Go web application running in Docker. The Dockerfile works but has room for improvement.

Review the Dockerfile, identify the issues you see, and apply your Docker skills to make it production-ready. Think about image size, build strategy, security, and what makes Go uniquely suited for minimal container images. Make it something you would be confident shipping.

## Build and run

```bash
docker build -t go-app .
docker run --rm -d -p 8080:8080 go-app
```

## Verify

```bash
curl http://docker.internal:8080
```
