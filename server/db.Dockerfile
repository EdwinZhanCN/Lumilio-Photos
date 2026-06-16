# syntax=docker/dockerfile:1

FROM pgvector/pgvector:pg17 AS builder

RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    ca-certificates \
    curl \
    git \
    postgresql-server-dev-17 \
  && rm -rf /var/lib/apt/lists/*

# pg_textsearch: native BM25 index access method
RUN git clone --depth 1 https://github.com/timescale/pg_textsearch.git /tmp/pg_textsearch \
  && cd /tmp/pg_textsearch \
  && make PG_CONFIG=/usr/bin/pg_config \
  && make PG_CONFIG=/usr/bin/pg_config install

# SCWS: Chinese word segmentation library required by zhparser
RUN curl -fsSL http://www.xunsearch.com/scws/down/scws-1.2.3.tar.bz2 -o /tmp/scws.tar.bz2 \
  && cd /tmp && tar xf scws.tar.bz2 \
  && cd scws-1.2.3 \
  && ./configure --prefix=/usr/local/scws \
  && make -j"$(nproc)" && make install

# zhparser: PostgreSQL Chinese full-text search parser
RUN git clone --depth 1 https://github.com/amutu/zhparser.git /tmp/zhparser \
  && cd /tmp/zhparser \
  && SCWS_HOME=/usr/local/scws make PG_CONFIG=/usr/bin/pg_config \
  && SCWS_HOME=/usr/local/scws make PG_CONFIG=/usr/bin/pg_config install

FROM pgvector/pgvector:pg17

LABEL org.opencontainers.image.title="lumilio-dev-db" \
      org.opencontainers.image.description="Development PostgreSQL 17 + pgvector + pg_textsearch + zhparser image for Lumilio Photos"

# Copy compiled extensions from builder
COPY --from=builder /usr/share/postgresql/17/extension/pg_textsearch* /usr/share/postgresql/17/extension/
COPY --from=builder /usr/lib/postgresql/17/lib/pg_textsearch* /usr/lib/postgresql/17/lib/
COPY --from=builder /usr/share/postgresql/17/extension/zhparser* /usr/share/postgresql/17/extension/
COPY --from=builder /usr/lib/postgresql/17/lib/zhparser* /usr/lib/postgresql/17/lib/
COPY --from=builder /usr/local/scws/lib/ /usr/local/scws/lib/
COPY --from=builder /usr/local/scws/etc/ /usr/local/scws/etc/

# SCWS shared library must be findable at runtime
RUN echo "/usr/local/scws/lib" > /etc/ld.so.conf.d/scws.conf && ldconfig

HEALTHCHECK --interval=10s --timeout=5s --retries=5 CMD \
  pg_isready -U "$${POSTGRES_USER:-postgres}" -d "$${POSTGRES_DB:-lumiliophotos}" || exit 1
