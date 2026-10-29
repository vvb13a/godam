#!/bin/sh
set -eu

ttl="${GODAM_DELIVERY_CACHE_TTL:-86400}"
case "$ttl" in
  ''|*[!0-9]*) ttl=86400 ;;
esac
if [ "$ttl" -lt 60 ]; then ttl=60; fi
if [ "$ttl" -gt 604800 ]; then ttl=604800; fi
export GODAM_DELIVERY_CACHE_TTL="$ttl"

exec caddy run --config /etc/caddy/Caddyfile --adapter caddyfile
