#!/usr/bin/env bash
set -euo pipefail

DATA_DIR="${NATURAL_EARTH_DATA_DIR:-/data/naturalearth}"
DOWNLOAD_DIR="${DATA_DIR}/downloads"
EXTRACT_DIR="${DATA_DIR}/extracted"
FORCE=false

if [[ "${1:-}" == "--force" ]]; then
  FORCE=true
fi

mkdir -p "${DOWNLOAD_DIR}" "${EXTRACT_DIR}"

pg_uri() {
  printf 'host=%s port=%s dbname=%s user=%s password=%s' \
    "${PGHOST:-db}" \
    "${PGPORT:-5432}" \
    "${PGDATABASE:-lumiliophotos}" \
    "${PGUSER:-postgres}" \
    "${PGPASSWORD:-postgres}"
}

psql_exec() {
  psql -v ON_ERROR_STOP=1 "$@"
}

download_layer() {
  local layer="$1"
  local url="$2"
  local zip_path="${DOWNLOAD_DIR}/${layer}.zip"

  if [[ -f "${zip_path}" && "${FORCE}" != true ]]; then
    echo "==> ${layer}: using cached ${zip_path}"
    return
  fi

  echo "==> ${layer}: downloading ${url}"
  curl -fL --retry 3 --retry-delay 2 "${url}" -o "${zip_path}.tmp"
  mv "${zip_path}.tmp" "${zip_path}"
}

extract_layer() {
  local layer="$1"
  local zip_path="${DOWNLOAD_DIR}/${layer}.zip"
  local layer_dir="${EXTRACT_DIR}/${layer}"

  if [[ -d "${layer_dir}" && "${FORCE}" != true ]]; then
    echo "==> ${layer}: using cached extraction"
    return
  fi

  echo "==> ${layer}: extracting"
  rm -rf "${layer_dir}"
  mkdir -p "${layer_dir}"
  unzip -q "${zip_path}" -d "${layer_dir}"
}

table_has_rows() {
  local table="$1"
  local exists
  exists="$(psql -Atqc "SELECT to_regclass('public.${table}') IS NOT NULL")"
  if [[ "${exists}" != "t" ]]; then
    return 1
  fi

  local has_rows
  has_rows="$(psql -Atqc "SELECT EXISTS (SELECT 1 FROM public.${table} LIMIT 1)")"
  [[ "${has_rows}" == "t" ]]
}

find_shapefile() {
  local layer="$1"
  local shp
  shp="$(find "${EXTRACT_DIR}/${layer}" -name "${layer}.shp" -print -quit)"
  if [[ -z "${shp}" ]]; then
    echo "Missing shapefile for ${layer}" >&2
    exit 1
  fi
  printf '%s\n' "${shp}"
}

import_layer() {
  local layer="$1"
  local table="$2"
  local geometry_type="$3"

  if table_has_rows "${table}" && [[ "${FORCE}" != true ]]; then
    echo "==> ${table}: already imported"
    return
  fi

  local shp
  shp="$(find_shapefile "${layer}")"

  echo "==> ${table}: importing ${shp}"
  ogr2ogr \
    -f PostgreSQL "PG:$(pg_uri)" \
    "${shp}" \
    -nln "${table}" \
    -lco GEOMETRY_NAME=geom \
    -lco FID=gid \
    -lco PRECISION=NO \
    -nlt "${geometry_type}" \
    -overwrite
}

create_indexes() {
  echo "==> Creating Natural Earth indexes"
  psql_exec <<'SQL'
CREATE INDEX IF NOT EXISTS idx_natural_earth_admin0_geom
  ON public.natural_earth_admin0 USING GIST (geom);
CREATE INDEX IF NOT EXISTS idx_natural_earth_admin1_geom
  ON public.natural_earth_admin1 USING GIST (geom);
CREATE INDEX IF NOT EXISTS idx_natural_earth_populated_places_geom
  ON public.natural_earth_populated_places USING GIST (geom);

ANALYZE public.natural_earth_admin0;
ANALYZE public.natural_earth_admin1;
ANALYZE public.natural_earth_populated_places;
SQL
}

echo "==> Ensuring PostGIS extension exists"
psql_exec -c "CREATE EXTENSION IF NOT EXISTS postgis;"

download_layer "ne_10m_admin_0_countries" "https://naciscdn.org/naturalearth/10m/cultural/ne_10m_admin_0_countries.zip"
download_layer "ne_10m_admin_1_states_provinces" "https://naciscdn.org/naturalearth/10m/cultural/ne_10m_admin_1_states_provinces.zip"
download_layer "ne_10m_populated_places" "https://naciscdn.org/naturalearth/10m/cultural/ne_10m_populated_places.zip"

extract_layer "ne_10m_admin_0_countries"
extract_layer "ne_10m_admin_1_states_provinces"
extract_layer "ne_10m_populated_places"

import_layer "ne_10m_admin_0_countries" "natural_earth_admin0" "MULTIPOLYGON"
import_layer "ne_10m_admin_1_states_provinces" "natural_earth_admin1" "MULTIPOLYGON"
import_layer "ne_10m_populated_places" "natural_earth_populated_places" "POINT"

create_indexes

echo "==> Natural Earth import complete"
