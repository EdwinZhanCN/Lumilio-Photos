# Build React app stage
FROM node:23.11.1-alpine as build

# Install pnpm
RUN npm install -g pnpm

WORKDIR /app

# Copy package.json and pnpm-lock.yaml files
COPY web/package.json web/pnpm-lock.yaml ./

# Install dependencies
RUN pnpm install --frozen-lockfile

# Copy frontend source code
COPY web/ ./

# Create env file at build time with placeholder
RUN echo "VITE_API_URL=RUNTIME_API_URL" > .env

# Build the application
RUN pnpm run build

# Production stage
FROM nginx:alpine

# Install openssl for self-signed cert generation
RUN apk add --no-cache openssl

# Create certs directory and generate self-signed certificate
RUN mkdir -p /etc/nginx/certs && \
    openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
    -keyout /etc/nginx/certs/selfsigned.key \
    -out /etc/nginx/certs/selfsigned.crt \
    -subj "/C=US/ST=Dev/L=Local/O=Dev/CN=localhost"

# Copy built files from build stage to nginx
COPY --from=build /app/dist /usr/share/nginx/html

# Add configuration for nginx
COPY web/nginx.conf /etc/nginx/conf.d/default.conf

# Create entrypoint script for runtime configuration
RUN echo '#!/bin/sh' > /docker-entrypoint.sh && \
    echo 'API_URL=${API_URL:-http://localhost:8080}' >> /docker-entrypoint.sh && \
    echo 'find /usr/share/nginx/html -name "*.js" -exec sed -i "s|RUNTIME_API_URL|$API_URL|g" {} \;' >> /docker-entrypoint.sh && \
    echo 'nginx -g "daemon off;"' >> /docker-entrypoint.sh && \
    chmod +x /docker-entrypoint.sh

# Expose ports (80 for HTTP, 443 for HTTPS)
EXPOSE 80 443

CMD ["/docker-entrypoint.sh"]
