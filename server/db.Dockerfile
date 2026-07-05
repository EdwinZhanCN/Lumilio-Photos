# syntax=docker/dockerfile:1

FROM pgvector/pgvector:pg17

LABEL org.opencontainers.image.title="lumilio-db" \
      org.opencontainers.image.description="PostgreSQL 17 + pgvector + pg_trgm for Lumilio Photos" \
      org.opencontainers.image.source="https://github.com/EdwinZhanCN/Lumilio-Photos"

HEALTHCHECK --interval=10s --timeout=5s --retries=5 CMD \
  pg_isready -U "$${POSTGRES_USER:-postgres}" -d "$${POSTGRES_DB:-lumiliophotos}" || exit 1
