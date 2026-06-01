#!/bin/sh
set -eu

API_URL=${API_URL-}
find /usr/share/caddy -name "*.js" -exec sed -i "s|RUNTIME_API_URL|${API_URL}|g" {} \;

if [ "$#" -eq 0 ]; then
	set -- caddy run --config /etc/caddy/Caddyfile --adapter caddyfile
fi

exec "$@"
