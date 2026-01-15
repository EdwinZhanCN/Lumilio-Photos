# Build React app stage
FROM node:22.3.0-alpine as build

WORKDIR /app

# Copy package.json files
COPY web/package*.json ./

# Install dependencies
RUN npm ci

# Copy frontend source code
COPY web/ ./

# Create env file at build time with placeholder
RUN echo "VITE_API_URL=RUNTIME_API_URL" > .env

# Build the application
RUN npm run build

# Production stage
FROM nginx:alpine

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

# Expose port
EXPOSE 80

CMD ["/docker-entrypoint.sh"]
