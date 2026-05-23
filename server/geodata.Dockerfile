# syntax=docker/dockerfile:1

FROM ghcr.io/osgeo/gdal:ubuntu-small-latest

LABEL org.opencontainers.image.title="Lumilio Natural Earth Importer"
LABEL org.opencontainers.image.description="GDAL tooling for importing Natural Earth shapefiles into PostGIS"

RUN apt-get update \
  && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    postgresql-client \
    unzip \
  && rm -rf /var/lib/apt/lists/*

COPY server/scripts/import-natural-earth.sh /import-natural-earth.sh
RUN chmod +x /import-natural-earth.sh
