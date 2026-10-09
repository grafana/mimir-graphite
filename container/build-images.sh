#!/bin/sh

MIMIR_PROXY_SOURCE="$(realpath $(dirname $0)/..)"

MIMIR_PROXY_VERSION="$(grep -oP '\b\d+\.\d+\.\d+\b' ${MIMIR_PROXY_SOURCE}/.release-please-manifest.json)"

docker build \
  --no-cache-filter 'mimir-proxy' \
  -t "grafana/mimir-proxies:${MIMIR_PROXY_VERSION}" \
  --build-arg="IMAGE_BASE=alpine:latest" \
  -f "${MIMIR_PROXY_SOURCE}/container/Dockerfile" \
  "${MIMIR_PROXY_SOURCE}"

docker build \
  --no-cache-filter 'mimir-proxy' \
  -t "grafana/mimir-proxies:${MIMIR_PROXY_VERSION}-memcached" \
  --build-arg="IMAGE_BASE=memcached:alpine" \
  -f "${MIMIR_PROXY_SOURCE}/container/Dockerfile" \
  "${MIMIR_PROXY_SOURCE}"

docker rmi $(docker images -f "dangling=true" -q)

