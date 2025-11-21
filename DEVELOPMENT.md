# Development Guide

## 🛠 Local Development Setup

### Prerequisites
*   **Go**: Version 1.22 or higher.
*   **Docker**: For running dependencies (Postgres, Redis).
*   **Node.js** (Optional): Only if you plan to modify Tailwind CSS significantly (though the current setup uses CDN/Vanilla for simplicity).

### 1. Start Dependencies
Use Docker Compose to start PostgreSQL and Redis without running the app container:

```bash
docker-compose up -d db redis
```

### 2. Environment Setup
Copy the example environment file:

```bash
cp .env.example .env
```

Edit `.env` to match your local Docker ports (usually default):
```env
DATABASE_URL=postgres://postgres:postgres@localhost:5432/sentinel?sslmode=disable
REDIS_ADDR=localhost:6379
```

### 3. Run the Application
Run the Go server with hot-reload (if you have `air` installed) or standard `go run`:

```bash
# Standard run
go run main.go

# Or with Air (for live reload)
# go install github.com/cosmtrek/air@latest
air
```

The app will be available at `http://localhost:8080`.

## 🧪 Testing

### Manual Testing
*   **Login**: Use `admin` / `admin123`.
*   **Webhooks**: Use `curl` to simulate alerts.
    ```bash
    curl -X POST http://localhost:8080/webhook \
      -d '{"title":"Test","message":"Hello World","level":"info"}'
    ```

### Automated Tests
(Coming Soon)

## 🎨 Frontend Development
The frontend uses **Tailwind CSS** via CDN for rapid prototyping.
*   **Templates**: Located in `web/templates/`.
*   **Static Assets**: Located in `web/static/` (CSS, JS, Icons).
*   **Service Worker**: `web/static/sw.js`.

To update the UI:
1.  Modify `web/templates/index.html`.
2.  Refresh the browser.

## 📦 Database Migrations
The application runs migrations automatically on startup (`RunMigrations` in `postgres.go`).
*   Schema definitions are in `internal/store/schema.sql`.
*   To add a new table, update `schema.sql` and add the migration logic in `RunMigrations`.
