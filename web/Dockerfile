FROM node:22.3.0-alpine as build

WORKDIR /app

COPY web/package*.json ./

RUN npm ci

COPY web/ ./

# Create env file at build time with the correct backend URL
# This assumes the backend is available at 'app:8080' inside the Docker network
RUN echo "VITE_API_URL=http://app:8080" > .env

RUN npm run build

FROM nginx:alpine

COPY --from=build /app/build /usr/share/nginx/html

# Add configuration to allow runtime environment variables (optional)
COPY web/nginx.conf /etc/nginx/templates/default.conf.template

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]