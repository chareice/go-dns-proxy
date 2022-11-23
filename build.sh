#! /bin/bash
docker buildx build --platform linux/amd64 --push -t ghcr.io/chareice/go-dns-proxy:latest .