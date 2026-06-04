ARG IMAGE_BASE=alpine:latest
ARG GO_VERSION=1.20.5

FROM golang:${GO_VERSION}-alpine AS builder

RUN apk add bash git \
      && git clone --depth=1 https://github.com/grafana/mimir-proxies.git \
      && cd mimir-proxies \
      && bash ./scripts/compile_commands.sh

FROM ${IMAGE_BASE} as minir-proxy

USER root

RUN set -eux \
  && addgroup -S -g 1000 mimir-proxy \
  && adduser -u 1000 -D -G mimir-proxy mimir-proxy \
  && mkdir -p /var/local/mimir-proxies \
  && mkdir -p /var/log/mimir-proxies \
  && chown mimir-proxy:mimir-proxy /var/local/mimir-proxies \
  && chown mimir-proxy:mimir-proxy /var/log/mimir-proxies

WORKDIR /var/local/mimir-proxies

COPY --from=builder /go/mimir-proxies/dist/*-proxy-writes /usr/local/bin/

COPY container/docker-entrypoint.sh /usr/local/bin/

ENV JAEGER_DISABLED=true

ENTRYPOINT [ "docker-entrypoint.sh" ]

USER 1000:1000

EXPOSE 9009

CMD [ "datadog" ]

