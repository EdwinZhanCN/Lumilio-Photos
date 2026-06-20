# syntax=docker/dockerfile:1

FROM pgvector/pgvector:pg17

LABEL org.opencontainers.image.title="lumilio-dev-db" \
      org.opencontainers.image.description="Development PostgreSQL 17 + pgvector + pg_trgm image for Lumilio Photos"

HEALTHCHECK --interval=10s --timeout=5s --retries=5 CMD \
  pg_isready -U "$${POSTGRES_USER:-postgres}" -d "$${POSTGRES_DB:-lumiliophotos}" || exit 1
