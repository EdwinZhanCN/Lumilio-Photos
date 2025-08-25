# RiverQueue

## Create Worker

## Add New Queue to Setup

## Migration Commands

Export Your PostgreSQL Database Connection String

```shell
export DATABASE_URL="connstring"
```

Migrate Up (Include in internal/db/migration.go)

```shell
river migrate-up --line main --database-url "$DATABASE_URL"
```

Migrate Down

```shell
river migrate-down --line main --database-url "$DATABASE_URL" --max-steps 10
```
