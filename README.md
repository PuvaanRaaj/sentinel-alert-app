# Sentinel Alert App (Incident Viewer)

A real-time incident alert viewer and management system built with Go, PostgreSQL, and Redis.

![alt text](dashboard.png)



## Features

### 🚀 Real-Time Monitoring
- **Live Alerts**: Real-time incident updates using Server-Sent Events (SSE).
- **Multi-Channel Support**: Organize alerts into different chat channels.
- **Responsive UI**: Modern dashboard built with Tailwind CSS.

### 🔐 Security & User Management
- **Role-Based Access Control**:
  - **Admins**: Full access to all chats, user management, and system settings.
  - **Users**: Restricted access to assigned chats only.
- **Two-Factor Authentication (2FA)**:
  - Mandatory for Admin users.
  - Optional for Regular users.
  - Time-based One-Time Password (TOTP) support (Google Authenticator, Authy, etc.).
- **User Management**:
  - Create/Edit users.
  - Assign specific chat permissions.
  - Password management (Change password, Admin reset).
  - Profile updates.

### 🤖 Integration & System
- **Bot Integration**: Create bots to push alerts to specific chats.
- **Webhook Support**: Simple API for external services to push alerts.
- **Alert Management**: System-wide alert purging capability.
- **Persistent Storage**: PostgreSQL for data, Redis for high-speed alert caching.

## Tech Stack

- **Backend**: Go (Golang)
- **Database**: PostgreSQL (Supabase compatible)
- **Cache/PubSub**: Redis
- **Frontend**: HTML5, Vanilla JavaScript, Tailwind CSS
- **Containerization**: Docker & Docker Compose

## Getting Started

### Prerequisites
- Go 1.21+
- Docker & Docker Compose
- PostgreSQL
- Redis

### Environment Variables
Create a `.env` file in the root directory:

```env
PORT=8080
DATABASE_URL=postgres://user:password@localhost:5432/dbname?sslmode=disable
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=
REDIS_DB=0
```

### Running with Docker Compose (Recommended)

```bash
docker-compose up --build
```

### Running Locally

1. Start Redis and PostgreSQL.
2. Run the application:
```bash
go run main.go
```

## API Documentation

### Authentication
- `POST /api/login` - Public login (returns session & allowed chats)
- `POST /api/login/verify-2fa` - Verify 2FA code

### User Management
- `PUT /api/user/profile` - Update profile
- `POST /api/user/change-password` - Change password
- `POST /api/user/2fa/generate` - Generate 2FA secret
- `POST /api/user/2fa/enable` - Enable 2FA

### Admin API
- `POST /api/admin/users` - Create user
- `PUT /api/admin/users/{id}` - Update user
- `POST /api/admin/reset-password` - Reset user password
- `POST /api/admin/purge` - Purge all alerts

### Webhooks
- `POST /bot/{token}` - Push alert to chat
  ```json
  {
    "level": "error",
    "title": "System Down",
    "message": "Server X is not responding"
  }
  ```

## Default Credentials
- **Username**: `admin`
- **Password**: `admin123`
