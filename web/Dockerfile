## Build WASM stage
#FROM rust:1.85-slim as wasm-build
#
## Install required dependencies for wasm-pack
#RUN apt-get update && apt-get install -y curl pkg-config libssl-dev
#
## Install wasm-pack
#RUN curl https://rustwasm.github.io/wasm-pack/installer/init.sh -sSf | sh || \
#    (echo "Failed to install wasm-pack via script, trying cargo install" && \
#    cargo install wasm-pack)
#
#WORKDIR /wasm
#COPY web/thumbnail-wasm/ ./
#
## Build the WASM package
#RUN wasm-pack build --target web --out-dir pkg

# Build React app stage
FROM node:22.3.0-alpine as build

WORKDIR /app

# Copy package.json files
COPY web/package*.json ./

# Install dependencies
RUN npm ci

# Copy frontend source code
COPY web/ ./

# Copy WASM build output to match the path in vite.config.mts
#COPY --from=wasm-build /wasm/pkg/ ./thumbnail-wasm/pkg/

# Create env file at build time
RUN echo "VITE_API_URL=http://app:8080" > .env

# Build the application
RUN npm run build

# Production stage
FROM nginx:alpine

# Copy built files from build stage to nginx
COPY --from=build /app/dist /usr/share/nginx/html

# Add configuration for nginx
COPY web/nginx.conf /etc/nginx/conf.d/default.conf

# Expose port
EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]