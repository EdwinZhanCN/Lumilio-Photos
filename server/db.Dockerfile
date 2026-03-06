# syntax=docker/dockerfile:1

FROM pgvector/pgvector:pg16

LABEL org.opencontainers.image.title="lumilio-dev-db" \
      org.opencontainers.image.description="Development PostgreSQL 16 + pgvector image for Lumilio Photos"

HEALTHCHECK --interval=10s --timeout=5s --retries=5 CMD \
  pg_isready -U "$${POSTGRES_USER:-postgres}" -d "$${POSTGRES_DB:-lumiliophotos}" || exit 1
