# Build React app stage
FROM node:23.11.1-bookworm-slim as build

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
FROM nginx:alpine

# Copy built files from build stage to nginx
COPY --from=build /app/dist /usr/share/nginx/html

# Add configuration for nginx
COPY web/nginx.conf /etc/nginx/conf.d/default.conf

# Create entrypoint script for runtime configuration
RUN echo '#!/bin/sh' > /docker-entrypoint.sh && \
    echo 'API_URL=${API_URL-}' >> /docker-entrypoint.sh && \
    echo 'find /usr/share/nginx/html -name "*.js" -exec sed -i "s|RUNTIME_API_URL|$API_URL|g" {} \;' >> /docker-entrypoint.sh && \
    echo 'exec nginx -g "daemon off;"' >> /docker-entrypoint.sh && \
    chmod +x /docker-entrypoint.sh

# Expose HTTP. TLS/HTTP2 should be handled by an external reverse proxy.
EXPOSE 80

ENTRYPOINT []
CMD ["/docker-entrypoint.sh"]
