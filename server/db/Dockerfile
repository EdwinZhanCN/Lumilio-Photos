# Dockerfile
FROM postgres:latest

# ——— 安装 pgvector ———
# 1. 安装编译依赖
# 2. 从 GitHub 拉取 pgvector 源码并编译安装
RUN apt-get update && apt-get install -y \
    build-essential \
    git \
    postgresql-server-dev-all \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /tmp
RUN git clone https://github.com/pgvector/pgvector.git

WORKDIR /tmp/pgvector
RUN make
RUN make install

EXPOSE 5432