#!/bin/sh
set -e

SCRIPT_PATH="$(dirname $(realpath $0))"

if [ -z "$PROXY_TYPE" ]; then
  PROXY_TYPE="$1"
fi

PROXY_COMMAND="${PROXY_TYPE}-proxy-writes"

if command -v "${PROXY_COMMAND}" >/dev/null; then
  if command -v "memcached" >/dev/null; then
    echo "Using $(memcached --version)"
    memcached $MEMCACHED_PARAMS >> /dev/null 2>&1 &
    PROXY_MEMCACHE_SERVER=127.0.0.1:11211
  fi

  echo "Proxy type ${PROXY_TYPE}"
  if [[ ${#} -gt 1 ]]; then
    set -- $PROXY_COMMAND"${@//$PROXY_TYPE/}"
  else
    MIMIR_API="${PROXY_ENTRY_PROTOCOL:-http}://${PROXY_ENTRY_SERVER:-mimir}:${POXY_ENTRY_PORT:-9009}/api"
    set -- $PROXY_COMMAND \
                          -server.http-listen-address=${PROXY_LISTENER_ADDRESS:-0.0.0.0} \
                          -server.http-listen-port=${PROXY_LISTENER_PORT:-8009} \
                          -server.path-prefix="${PROXY_PREFIX:-"/${PROXY_TYPE}/"}" \
                          -auth.enable="${PROXY_AUTH:-false}" \
                          -memcached-server "${PROXY_MEMCACHE_SERVER:-memcached:11211}" \
                          -write-endpoint "${MIMIR_API}/prom/push" \
                          -query-endpoint "${MIMIR_API}/prom"
  fi
  echo "Running:"
  echo "$@"
  echo "Listener: http://$(hostname -i):${PROXY_LISTENER_PORT}/${PROXY_TYPE}/"
fi

exec "$@"

