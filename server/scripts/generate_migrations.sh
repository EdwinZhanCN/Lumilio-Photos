#!/bin/bash

# Generate database migrations from schema files
# Usage: ./generate_migrations.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCHEMA_DIR="$SCRIPT_DIR/../schema"
MIGRATIONS_DIR="$SCRIPT_DIR/../migrations"

echo "Generating migrations from schema files..."
echo ""

# Create migrations directory if it doesn't exist
mkdir -p "$MIGRATIONS_DIR"

# Get current timestamp
TIMESTAMP=$(date +%s)

# Process each schema file in order
for schema_file in "$SCHEMA_DIR"/*.sql; do
    if [ ! -f "$schema_file" ]; then
        continue
    fi

    filename=$(basename "$schema_file")
    base_name="${filename%.sql}"

    echo "Processing: $filename"

    # Generate UP migration
    up_file="$MIGRATIONS_DIR/${TIMESTAMP}_${base_name}.up.sql"
    cp "$schema_file" "$up_file"
    echo "  Created: ${TIMESTAMP}_${base_name}.up.sql"

    # Generate DOWN migration
    down_file="$MIGRATIONS_DIR/${TIMESTAMP}_${base_name}.down.sql"

    # Extract and reverse objects from schema
    {
        echo "-- Reverse migration - drops all objects created in up migration"
        echo "-- WARNING: This will permanently delete data"
        echo ""

        # Extract triggers and reverse order
        grep -i "CREATE TRIGGER" "$schema_file" | sed -n 's/.*CREATE TRIGGER \([^ ]*\).*ON \([^ ]*\).*/DROP TRIGGER IF EXISTS \1 ON \2;/p' | tail -r

        # Extract functions and reverse order
        grep -i "CREATE.*FUNCTION" "$schema_file" | sed -n 's/.*FUNCTION \([^(]*\).*/DROP FUNCTION IF EXISTS \1();/p' | tail -r

        # Extract indexes and reverse order
        grep -i "CREATE.*INDEX" "$schema_file" | grep -o "INDEX[^O]*ON" | awk '{print $2}' | sed 's/$/;/' | sed 's/^/DROP INDEX IF EXISTS /' | tail -r

        # Extract tables and reverse order
        grep -i "CREATE TABLE" "$schema_file" | grep -o "CREATE TABLE[^(]*" | sed 's/CREATE TABLE IF NOT EXISTS//g' | sed 's/CREATE TABLE//g' | awk '{print $NF}' | sed 's/^/DROP TABLE IF EXISTS /' | sed 's/$/ CASCADE;/' | tail -r

        # Extract extensions and reverse order
        grep -i "CREATE EXTENSION" "$schema_file" | sed 's/CREATE EXTENSION IF NOT EXISTS/DROP EXTENSION IF EXISTS/g' | sed 's/CREATE EXTENSION/DROP EXTENSION IF EXISTS/g' | grep -o "DROP EXTENSION[^;]*" | sed 's/$/ CASCADE;/' | tail -r
    } > "$down_file"

    echo "  Created: ${TIMESTAMP}_${base_name}.down.sql"
    echo ""

    # Increment timestamp for next file
    TIMESTAMP=$((TIMESTAMP + 1))
done

echo "Migration generation complete!"
echo "Files created in: $MIGRATIONS_DIR"
