# Containerization of Mimir Proxies

## Build

```shell
/usr/local/src$ git clone --depth=1 https://github.com/grafana/mimir-proxies.git
/usr/local/src$ cd mimir-proxies
/usr/local/src/mimir-proxies$ ./container/build-image.sh
```

## Running

All-in-one:
```shell
/usr/local/src/mimir-proxies$ cd container
/usr/local/src/mimir-proxies/container$ docker-compose up -d
```

Mimir proxy with memcached:
```shell
/usr/local/src/mimir-proxies$ docker run --name mimir_proxy_default grafana/mimir-proxies:memcached
/usr/local/src/mimir-proxies$ docker run --name mimir_proxy_default grafana/mimir-proxies:memcached graphite
```

Proxy without memcache:
```shell
/usr/local/src/mimir-proxies$ docker run --name mimir_proxy_default grafana/mimir-proxies:simple
/usr/local/src/mimir-proxies$ docker run --name mimir_proxy_default grafana/mimir-proxies:simple graphite
```

### Enviroments

- **PROXY_TYPE**: Type of proxy, defautl datadog, options datadog|graphite
- **PROXY_MEMCACHE_SERVER**: IP/Hostname with port of memcached, default memcached, foced to local (127.0.0.1:11211) in container mimir-proxies:memcached
- **PROXY_ENTRY_PROTOCOL**: Protocol of entri point of Mimir, default 9009
- **PROXY_ENTRY_SERVER**: Server name or IP of server Mimir, default mimir
- **PROXY_LISTENER_ADDRESS**: Listener ip of the proxy, default 0.0.0.0 (all)
- **PROXY_LISTENER_PORT**: Listener port of proxy, default 8009
- **PROXY_PREFIX**: Path of proxy api, defautl like ``PROXY_TYPE``
- **PROXY_AUTH**: Enabled or not of auth, true|false

