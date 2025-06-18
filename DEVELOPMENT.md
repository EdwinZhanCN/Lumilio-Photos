# Lumilio Photos - Development Setup

This guide will help you set up the Lumilio Photos application for local development.

## Prerequisites

- Go 1.21 or later
- Docker and Docker Compose
- PostgreSQL client (optional, for database access)
- Git

## Quick Start

### 1. Clone and Setup

```bash
git clone <repository-url>
cd Lumilio
```

### 2. Environment Configuration

Copy the development environment template:

```bash
cd server
cp .env.development .env
```

The `.env` file contains development-friendly defaults:
- Database connects to `localhost:5432`
- Storage uses local directories (`./data/photos`, `./staging`, `./queue`)
- Filename preservation is enabled
- Date-based storage strategy for easy browsing

### 3. Start Database Only

For development, you only need the database running in Docker:

```bash
# From the project root
docker-compose up db -d
```

This starts PostgreSQL on `localhost:5432` with:
- Database: `lumiliophotos`
- Username: `postgres` 
- Password: `postgres`

### 4. Run the API Server

```bash
cd server
go mod download
go run cmd/api/main.go
```

The server will:
- ✅ Auto-detect development mode (`ENV=development`)
- ✅ Connect to localhost database
- ✅ Create local directories automatically
- ✅ Use filename-preserving storage
- ✅ Start on `http://localhost:8080`

### 5. Access the Application

- **API**: http://localhost:8080
- **API Documentation**: http://localhost:8080/swagger/index.html
- **Health Check**: http://localhost:8080/api/v1/health

## Development Features

### Automatic Environment Detection

The application automatically detects development mode when:
- `ENV=development`
- `ENVIRONMENT=development` 
- `DEV_MODE=true`

### Development vs Production Differences

| Feature | Development | Production |
|---------|-------------|------------|
| Database Host | `localhost` | `db` (Docker service) |
| Storage Path | `./data/photos` | `/app/data/photos` |
| Staging Path | `./staging` | `/app/staging` |
| Queue Path | `./queue` | `/app/queue` |
| File Preservation | ✅ Enabled | ✅ Enabled |
| Duplicate Handling | Rename with (1), (2) | Rename with (1), (2) |

### Storage Configuration

The development setup uses:

```yaml
Storage Strategy: date           # Files organized as YYYY/MM/filename.ext
Preserve Filenames: true         # Keep original filenames
Duplicate Handling: rename       # Add (1), (2) suffixes for duplicates
Base Path: ./data/photos         # Local development directory
```

### File Organization Example

```
./data/photos/
├── 2024/
│   ├── 01/
│   │   ├── vacation-photo.jpg
│   │   ├── vacation-photo (1).jpg  # Duplicate handling
│   │   └── document.pdf
│   └── 02/
│       └── new-photo.png
└── 2025/
    └── 01/
        └── latest-upload.jpg
```

## Database Management

### Connect to Development Database

```bash
# Using psql
psql -h localhost -p 5432 -U postgres -d lumiliophotos

# Using Docker
docker-compose exec db psql -U postgres -d lumiliophotos
```

### Reset Database

```bash
# Stop and remove database
docker-compose down -v

# Start fresh database
docker-compose up db -d

# Restart API (migrations run automatically)
go run cmd/api/main.go
```

### View Database Schema

```sql
-- List all tables
\dt

-- View asset table structure
\d assets

-- Check recent uploads
SELECT asset_id, original_filename, storage_path, upload_time 
FROM assets 
ORDER BY upload_time DESC 
LIMIT 10;
```

## API Testing

### Upload a File

```bash
curl -X POST http://localhost:8080/api/v1/assets/upload \
  -F "file=@/path/to/your/photo.jpg" \
  -H "Content-Type: multipart/form-data"
```

### List Assets

```bash
curl http://localhost:8080/api/v1/assets
```

### Search Assets

```bash
curl "http://localhost:8080/api/v1/assets/search?query=vacation&type=photo"
```

## Development Workflow

### 1. Making Changes

1. Edit Go files in `server/`
2. Restart the API server: `go run cmd/api/main.go`
3. Test changes via API or Swagger UI

### 2. Database Changes

1. Update models in `server/internal/models/`
2. Restart API (auto-migration will run)
3. Verify schema changes in database

### 3. Storage Changes

1. Modify storage logic in `server/internal/storage/`
2. Test with file uploads
3. Check file organization in `./data/photos/`

## Troubleshooting

### Database Connection Issues

```bash
# Check if database is running
docker-compose ps db

# Check database logs
docker-compose logs db

# Restart database
docker-compose restart db
```

### Storage Issues

```bash
# Check directory permissions
ls -la ./data/photos/

# Manually create directories
mkdir -p ./data/photos ./staging ./queue

# Check disk space
df -h .
```

### API Issues

```bash
# Check if port is available
lsof -i :8080

# View API logs (run in foreground)
go run cmd/api/main.go

# Check environment variables
env | grep -E "(DB_|STORAGE_|ENV)"
```

### Common Error Solutions

| Error | Solution |
|-------|----------|
| `connection refused` | Start database: `docker-compose up db -d` |
| `permission denied` | Check directory permissions: `chmod 755 ./data` |
| `port already in use` | Kill process: `lsof -ti:8080 \| xargs kill` |
| `migration failed` | Reset database and restart API |

## Production Deployment

To deploy to production:

1. Set `ENV=production` (or remove development env vars)
2. Use full docker-compose: `docker-compose up -d`
3. Access via configured ports/domains

The application will automatically use production settings:
- Database connects to `db` service
- Uses container paths (`/app/data/photos`)
- All services run in Docker network

## Contributing

1. Follow the development setup above
2. Make changes in feature branches
3. Test thoroughly with file uploads
4. Ensure database migrations work
5. Update documentation if needed

## Support

For issues or questions:
1. Check this development guide
2. Review API documentation at `/swagger/`
3. Check database schema and data
4. Review application logs