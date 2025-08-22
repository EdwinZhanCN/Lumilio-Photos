//
// Atlas configuration for Lumilio Photos using SQL files as the desired schema.
// Environment naming aligns with runtime config:
// - dev  = developer environment (e.g., Neon)
// - prod = production environment
//
// Usage examples (provide the DB URL at runtime):
//   atlas migrate diff   --env dev  --url "$NEON_DEV_DATABASE_URL"
//   atlas migrate apply  --env dev  --url "$NEON_DEV_DATABASE_URL"
//   atlas migrate status --env dev  --url "$NEON_DEV_DATABASE_URL"
//
//   atlas migrate diff   --env prod --url "$PROD_DATABASE_URL"
//   atlas migrate apply  --env prod --url "$PROD_DATABASE_URL"
//   atlas migrate status --env prod --url "$PROD_DATABASE_URL"
//
// Note on extensions:
// - The schema expects pgcrypto (for gen_random_uuid) and pgvector (for VECTOR type).
// - Extensions must be created manually as a separate migration before schema migrations.
// - The "dev" normalization database below uses a pgvector-enabled image to support CREATE EXTENSION vector.
//

env "dev" {
  // Desired schema lives in ./schema (relative to this file).
  src = "file://schema"

  // Dev normalization DB for planning diffs.
  // Uses a Postgres image with pgvector preinstalled to support the vector extension.
  dev = "docker://pgvector/pgvector/pg17/dev?search_path=public"

  // Migration directory (versioned migrations).
  migration {
    dir = "file://migrations"
  }

  // SQL output formatting for generated diffs.
  format {
    migrate {
      diff = "{{ sql . \"  \" }}"
    }
  }
}

env "prod" {
  // Desired schema lives in ./schema (relative to this file).
  src = "file://schema"

  // Dev normalization DB for planning diffs in production workflows.
  // Keep this separate from the actual production URL used at runtime.
  dev = "docker://pgvector/pgvector/pg17/prod?search_path=public"

  // Migration directory (versioned migrations).
  migration {
    dir = "file://migrations"
  }

  // SQL output formatting for generated diffs.
  format {
    migrate {
      diff = "{{ sql . \"  \" }}"
    }
  }
}
