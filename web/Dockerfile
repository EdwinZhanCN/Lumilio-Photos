# Build React app stage
FROM node:24.16.0-trixie-slim as build

# Install Vite+ and keep pnpm available for package-manager delegation.
RUN corepack enable \
    && corepack prepare pnpm@latest --activate \
    && apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl \
    && curl -fsSL https://vite.plus | bash \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

ENV PATH="/root/.vite-plus/bin:${PATH}"

WORKDIR /app

# Build-time plugin registry configuration for production bundle.
ARG VITE_PLUGIN_REGISTRY_URL=https://registry.lumilio.org
ARG VITE_PLUGIN_CDN_ORIGIN=https://cdn.lumilio.org
ARG VITE_STUDIO_PLUGIN_RUNTIME_ENABLED=true

# Copy package.json and pnpm-lock.yaml files
COPY web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml ./

# Install dependencies
RUN vp install

# Copy frontend source code
COPY web/ ./

# Create env file at build time.
RUN printf '%s\n' \
    "VITE_API_URL=RUNTIME_API_URL" \
    "VITE_PLUGIN_REGISTRY_URL=${VITE_PLUGIN_REGISTRY_URL}" \
    "VITE_PLUGIN_CDN_ORIGIN=${VITE_PLUGIN_CDN_ORIGIN}" \
    "VITE_STUDIO_PLUGIN_RUNTIME_ENABLED=${VITE_STUDIO_PLUGIN_RUNTIME_ENABLED}" \
    > .env

# Build the application
RUN vp build

# Production stage
FROM caddy:2-alpine

# Default to local plain HTTP. Set LUMILIO_SITE_ADDRESS to a real domain in
# production and Caddy will manage TLS certificates and HTTP/2 automatically.
ENV LUMILIO_SITE_ADDRESS=:80
ENV LUMILIO_API_UPSTREAM=h2c://server:6680

# Copy built files from build stage to Caddy's static root.
COPY --from=build /app/dist /usr/share/caddy

# Add Caddy reverse-proxy/static configuration and runtime env substitution.
COPY web/Caddyfile /etc/caddy/Caddyfile
COPY web/scripts/docker-entrypoint.sh /usr/bin/lumilio-web-entrypoint
RUN chmod +x /usr/bin/lumilio-web-entrypoint

# Expose HTTP and HTTPS. Caddy serves HTTP/2 on TLS listeners and h2c on :80.
EXPOSE 80
EXPOSE 443
EXPOSE 443/udp

ENTRYPOINT ["/usr/bin/lumilio-web-entrypoint"]
