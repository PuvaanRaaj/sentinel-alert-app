# Deployment Guide

This application is containerized using Docker, making it easy to deploy to any platform that supports Docker, such as **Render**, **Fly.io**, **Railway**, or **AWS ECS**.

## 🚀 Deploying to Render.com

Render is a great choice for Go applications.

### Option 1: Deploy as a Web Service (Docker)

1.  **Create a New Web Service** on Render.
2.  **Connect your GitHub repository**.
3.  **Select "Docker"** as the Runtime.
4.  **Configuration**:
    *   **Region**: Choose one close to you.
    *   **Branch**: `main`
    *   **Dockerfile Path**: `./Dockerfile` (default)
5.  **Environment Variables**:
    Add the following environment variables in the Render dashboard:
    *   `DATABASE_URL`: Your PostgreSQL connection string (Render provides a managed Postgres, or use Supabase/Neon).
    *   `REDIS_ADDR`: Your Redis address (Render provides managed Redis).
    *   `REDIS_PASSWORD`: (If applicable)
    *   `VAPID_SUBJECT`: `mailto:your-email@example.com`
    *   `VAPID_PUBLIC_KEY`: (Generate locally or let the app generate one and copy it from logs)
    *   `VAPID_PRIVATE_KEY`: (Same as above)
    *   `PORT`: `8080`

### Option 2: Deploy using `render.yaml` (Blueprint)

1.  Ensure `render.yaml` is in your repository root.
2.  In Render, go to **Blueprints** -> **New Blueprint Instance**.
3.  Connect your repo.
4.  Render will automatically detect the `web` service and `redis` (if defined).

## 🐳 Deploying with Docker Compose (VPS)

If you have a VPS (DigitalOcean, Linode, Hetzner) with Docker installed:

1.  **Clone the repository**:
    ```bash
    git clone https://github.com/yourusername/incident-viewer-go.git
    cd incident-viewer-go
    ```

2.  **Create `.env` file**:
    ```bash
    cp .env.example .env
    # Edit .env with your production values
    ```

3.  **Start the services**:
    ```bash
    docker-compose up -d --build
    ```

4.  **Verify**:
    ```bash
    docker-compose ps
    docker-compose logs -f
    ```

## 🔧 Post-Deployment

1.  **Admin User**:
    The app automatically seeds a default admin user:
    *   Username: `admin`
    *   Password: `admin123`
    *   **IMPORTANT**: Log in immediately and change this password!

2.  **VAPID Keys (Push Notifications)**:
    If you didn't provide `VAPID_PUBLIC_KEY` and `VAPID_PRIVATE_KEY`, the app will generate them on startup.
    *   Check the logs: `docker-compose logs app` (or Render logs).
    *   Copy the generated keys.
    *   Add them to your environment variables to persist them across restarts.
