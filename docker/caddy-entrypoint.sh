#!/bin/sh
set -eu

ttl="${GODAM_DELIVERY_CACHE_TTL:-86400}"
case "$ttl" in
  ''|*[!0-9]*) ttl=86400 ;;
esac
if [ "$ttl" -lt 60 ]; then ttl=60; fi
if [ "$ttl" -gt 604800 ]; then ttl=604800; fi
export GODAM_DELIVERY_CACHE_TTL="$ttl"

normalize_domain() {
  value="$1"
  value="${value#http://}"
  value="${value#https://}"
  value="${value%/}"
  printf '%s' "$value"
}

delivery_site=/tmp/godam-delivery.caddyfile
: > "$delivery_site"
domain="$(normalize_domain "${GODAM_DOMAIN:-}")"
delivery="$(normalize_domain "${GODAM_DELIVERY_DOMAIN:-$domain}")"
if [ -n "$delivery" ] && [ "$delivery" != "$domain" ]; then
  printf '%s {\n\timport delivery_routes\n}\n' "$delivery" > "$delivery_site"
fi

exec caddy run --config /etc/caddy/Caddyfile --adapter caddyfile
