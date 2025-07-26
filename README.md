>[!WARNING]
>🚧 This Project is Under Development. 🚧

For more information, see https://lumilio-photos-doc.vercel.app/

Learn detailed project structure and current progress, see https://deepwiki.com/EdwinZhanCN/Lumilio-Photos

## Development

### macOS

Install new [container](https://github.com/apple/container) tool for better performance.

**find `.env.development.example`**, duplicate it, and rename as your `.env.development`.

There are important field you need to modify
```
#...env
# By using Apple's container tool, it will assign a IP addr for your database
DB_HOST=localhost
DB_PORT=5432

#...

# Modify the path you want to store the photos, tasks, and queued photos
# Storage Configuration
STORAGE_PATH=/PATH/TO/YOUR/PHOTO
STAGING_PATH=/PATH/TO/YOUR/STAGING/FILE
QUEUE_DIR=/PATH/TO/YOUR/QUEUE_DIR

#...
```

**db**

Start standalone PostgreSQL Database, for apple's container tool, port will be diplayed by running `container ls`

```shell
container run -d --name db -e POSTGRES_PASSWORD=postgres -e POSTGRES_USER=postgres -e POSTGRES_DB=lumiliophotos postgres:15-alpine
```

**api**

start GIN api

```shell
go run server/cmd/api
```

**worker**

start worker for task processing

```shell
go run server/cmd/worker
```

**web**

Quick command under root directory

```shell
cd web && npm install && npm run dev
```
