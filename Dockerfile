# 构建后端阶段
FROM golang:1.23.3-alpine AS builder-backend
WORKDIR /app
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -o rkphoto-server ./cmd/main.go

# 构建前端阶段
FROM node:22.3.0-alpine AS builder-frontend
WORKDIR /app
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN echo "VITE_API_URL=/api" > .env
RUN npm run build

# 生产阶段
FROM alpine:latest

# 安装运行时依赖
RUN apk --no-cache add \
    ca-certificates \
    tzdata \
    exiftool \
    perl-image-exiftool

# 复制后端二进制
COPY --from=builder-backend /app/rkphoto-server /usr/local/bin/

# 复制前端构建结果
COPY --from=builder-frontend /app/dist /app/web/dist

# 创建存储目录并设置权限
RUN mkdir -p /app/data/photos && \
    chown -R nobody:nobody /app/data/photos && \
    chmod -R 755 /app/data/photos

# 设置环境变量
ENV STORAGE_PATH=/app/data/photos
ENV DB_HOST=postgres
ENV DB_PORT=5432
ENV DB_USER=dev
ENV DB_PASSWORD=Kr.lm<7knzb.;3^o

EXPOSE 8080
USER nobody

CMD ["/usr/local/bin/rkphoto-server"]