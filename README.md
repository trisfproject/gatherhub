# GatherHub

> **Modern Event Registration Platform** — A production-ready, full-stack event management system built with Go, Fiber, PostgreSQL, HTMX, and TailwindCSS.

---

## 📋 Project Overview

GatherHub is a streamlined event registration platform that enables organizers to create events and manage participant registrations with built-in payment verification workflows. Participants can register, upload payment proofs, and administrators can verify or reject registrations.

---

## ✨ Features

- **Event Management** — Create and manage events with full details (date, location, pricing, payment info)
- **Participant Registration** — Capture comprehensive attendee info (company, job title, Telegram, etc.)
- **Payment Verification Workflow** — PENDING → VERIFIED / REJECTED status lifecycle
- **File Upload Support** — Payment proofs and event banners stored in a configurable external volume
- **Participant Export** — One-click Excel (XLSX) export with filters
- **Health Monitoring** — Built-in `/health` endpoint with DB ping validation
- **Docker-First** — Full containerized setup with PostgreSQL, Go backend, and Nginx reverse proxy
- **Auto Migration** — GORM auto-migrates schema on startup, no manual SQL needed
- **RBAC** — Role-based access control: `SUPER_ADMIN` and `ADMIN` roles

---

## 🗂️ Folder Structure

```
~/Dev/event/
├── gatherhub/                  ← Git repository (no runtime uploads)
│   ├── backend/
│   │   ├── cmd/server/         # Application entrypoint (main.go)
│   │   ├── internal/
│   │   │   ├── config/         # Config loader + StorageConfig service
│   │   │   ├── database/       # GORM connection + auto-migration
│   │   │   ├── handlers/       # HTTP handlers (admin, pages, events)
│   │   │   ├── middleware/      # Auth + role middleware
│   │   │   ├── models/         # GORM models (Event, Participant, Admin)
│   │   │   ├── routes/         # Route registration
│   │   │   └── services/       # Business logic layer
│   │   ├── Dockerfile          # Multi-stage Docker build (Go 1.24)
│   │   └── go.mod
│   ├── frontend/               # HTMX + TailwindCSS pages
│   ├── nginx/                  # Nginx config + virtual host
│   ├── docker-compose.yml
│   ├── .env.example
│   └── README.md
│
└── runtime-storage/            # Runtime uploads (Ignored by Git, mounted in Docker)
    ├── payments/               # Payment proof uploads
    ├── events/                 # Event banner uploads
    └── temp/                   # Temporary files
```

> **Important:** `runtime-storage/` is inside the repository root but ignored via `.gitignore` so uploads are **never** committed to Git. Alternatively, `STORAGE_PATH` can point to any absolute path outside the repository.

---

## 📦 Storage Configuration

All runtime uploads are stored outside of Git tracking using a single environment variable:

```
STORAGE_PATH=/storage
```

Sub-directories are created automatically on startup:

| Path | Purpose |
|------|---------|
| `{STORAGE_PATH}/payments/` | Payment proof uploads (jpg, png, pdf) |
| `{STORAGE_PATH}/events/` | Event banner images |
| `{STORAGE_PATH}/temp/` | Temporary/scratch files |

### Storage Mounting & Configuration Examples

GatherHub's storage is completely environment-agnostic; it simply writes to and reads from `STORAGE_PATH` using standard Go file operations. This makes it compatible with any type of local or remote storage mounted at the system level:

#### 1. Local Development (Non-Docker)
Create a directory on your machine and set `STORAGE_PATH` to its absolute path:
```bash
export STORAGE_PATH=/home/username/Dev/event/gatherhub-storage
```

#### 2. Docker Volumes (Default)
In `docker-compose.yml`, mount a local folder into the container's `/storage` directory:
```yaml
services:
  backend:
    environment:
      STORAGE_PATH: /storage
    volumes:
      - ./runtime-storage:/storage
```

#### 3. Kubernetes Persistent Volume Claim (PVC)
Mount your PVC directly to the configured path inside the pod definition:
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: gatherhub-backend
spec:
  template:
    spec:
      containers:
      - name: backend
        image: gatherhub-backend:latest
        env:
        - name: STORAGE_PATH
          value: /var/lib/gatherhub/storage
        volumeMounts:
        - name: storage-volume
          mountPath: /var/lib/gatherhub/storage
      volumes:
      - name: storage-volume
        persistentVolumeClaim:
          claimName: gatherhub-pvc
```

#### 4. AWS EFS (Elastic File System)
EFS can be mounted on the host machine or container orchestration layers. For Docker Compose:
```yaml
volumes:
  efs-storage:
    driver: local
    driver_opts:
      type: nfs
      o: addr=fs-xxxxxx.efs.us-east-1.amazonaws.com,rw,nfsvers=4.1,rsize=1048576,wsize=1048576,hard,timeo=600,retrans=2,noresvport
      device: ":"

services:
  backend:
    environment:
      STORAGE_PATH: /storage
    volumes:
      - efs-storage:/storage
```

#### 5. NFS / SMB / CIFS Share
Mount the share at the OS level (e.g. via `/etc/fstab`) and set the `STORAGE_PATH` to the mount point:
```bash
# Example mounting NFS to /mnt/gatherhub-storage
mount -t nfs 192.168.1.100:/shares/gatherhub /mnt/gatherhub-storage
export STORAGE_PATH=/mnt/gatherhub-storage
```

---

## 🚀 Local Development (Without Docker)

### Prerequisites

- Go 1.24+
- PostgreSQL 17 running locally

### 1. Clone & configure

```bash
git clone <repository-url>
cd gatherhub

# Copy and edit environment variables
cp .env.example backend/.env
```

### 2. Create the storage directory

```bash
# One directory up from the repo root (outside Git)
mkdir -p ../gatherhub-storage/{payments,events,temp}
```

### 3. Edit `backend/.env`

```dotenv
STORAGE_PATH=../gatherhub-storage
```

The default value in `.env.example` already points to `../gatherhub-storage`, so this step is only needed if your layout differs.

### 4. Start PostgreSQL

Ensure PostgreSQL is running with:
```
DB: gatherhub  |  User: gatherhub  |  Password: gatherhub
```

### 5. Run the backend

```bash
cd backend
go mod download
go run cmd/server/main.go
```

The server will start on **http://localhost:3000**

### 6. Verify

```bash
curl http://localhost:3000/health
# → {"status":"ok"}
```

---

## 🐳 Docker Usage

### 1. Start all services

```bash
docker compose up -d
```

The compose file mounts `./runtime-storage` as `/storage` inside the backend container and `STORAGE_PATH=/storage` is set automatically. The directories `payments/`, `events/`, and `temp/` will be created automatically inside `./runtime-storage`.

| Service  | Port | Description              |
|----------|------|--------------------------|
| postgres | 5432 | PostgreSQL 17 database   |
| backend  | 3000 | Go + Fiber API server    |
| nginx    | 80   | Reverse proxy + frontend |

### 3. View logs

```bash
docker compose logs -f backend
docker compose logs -f postgres
```

### 4. Stop all services

```bash
docker compose down
```

### 5. Reset database (⚠️ deletes all data)

```bash
docker compose down -v
docker compose up -d
```

---

## ⚙️ Environment Variables

| Variable         | Default                | Description                                |
|------------------|------------------------|--------------------------------------------|
| `APP_PORT`       | `3000`                 | Server port                                |
| `APP_ENV`        | `development`          | Environment (`development` / `production`) |
| `DB_HOST`        | `localhost`            | Database host                              |
| `DB_PORT`        | `5432`                 | Database port                              |
| `DB_USER`        | `gatherhub`            | Database username                          |
| `DB_PASSWORD`    | `gatherhub`            | Database password                          |
| `DB_NAME`        | `gatherhub`            | Database name                              |
| `DB_SSLMODE`     | `disable`              | PostgreSQL SSL mode                        |
| `STORAGE_PATH`   | `/storage`             | **Root path for all runtime uploads**      |
| `FRONTEND_DIR`   | `../frontend`          | Frontend static files directory            |
| `ADMIN_USERNAME` | `admin`                | Default admin username (seed only)         |
| `ADMIN_PASSWORD` | `admin123`             | Default admin password (seed only)         |
| `SESSION_SECRET` | *(see example)*        | Session encryption key                     |

---

## 🔌 API Endpoints

| Method | Path                              | Description                    |
|--------|-----------------------------------|--------------------------------|
| GET    | `/health`                         | Health check + DB ping         |
| GET    | `/`                               | Landing page (active event)    |
| GET    | `/register`                       | Registration form              |
| POST   | `/register`                       | Submit registration            |
| GET    | `/admin/login`                    | Admin login page               |
| GET    | `/admin/dashboard`                | Admin dashboard                |
| GET    | `/admin/participants`             | Participant list (paginated)   |
| GET    | `/admin/participants/export`      | Export participants as XLSX    |
| GET    | `/admin/participants/:id`         | Participant detail             |
| POST   | `/admin/participants/:id/status`  | Update participant status      |
| GET    | `/admin/events`                   | Event management list          |
| GET    | `/admin/admins`                   | Admin user management (SUPER)  |

---

## 🌐 Production Deployment Guide

GatherHub is designed for robust, production-ready deployments. Here is how to configure Docker, Docker Compose, Nginx (as a Reverse Proxy), and NFS Shared Storage.

### 1. Standalone Docker Container

You can build and run the backend as a standalone container:

```bash
# Build the production image with version injection
docker build \
  --build-arg APP_VERSION=1.0.0 \
  --build-arg BUILD_DATE=$(date -u +'%Y-%m-%dT%H:%M:%SZ') \
  --build-arg GIT_COMMIT=$(git rev-parse HEAD) \
  -t gatherhub-backend:latest ./backend

# Run the container (injecting required variables)
docker run -d -p 3000:3000 \
  -e DB_HOST=192.168.1.10 \
  -e DB_PORT=5432 \
  -e DB_USER=gatherhub \
  -e DB_PASSWORD=secure_password \
  -e DB_NAME=gatherhub \
  -e STORAGE_PATH=/storage \
  -e SESSION_SECRET=super_secret_key_change_me \
  -e ADMIN_USERNAME=admin \
  -e ADMIN_PASSWORD=secure_admin_password \
  -v /mnt/shared-nfs-storage:/storage \
  gatherhub-backend:latest
```

### 2. Multi-Container Deployment via Docker Compose

In production, run the stack using Docker Compose:

```bash
# Start all containers in detached mode
docker compose up -d
```

Key features of this configuration:
* **Fail-Fast Startup**: The backend will wait until the Postgres DB container is fully healthy (`pg_isready` returns `0` inside its healthcheck) before initiating startup validation.
* **Eager Validation**: On startup, the backend verifies configuration, tests DB ping, and confirms that `/storage` is readable/writable. It fails fast if any validation fails.

### 3. Nginx Reverse Proxy & SSL Configuration

Deploy Nginx in front of GatherHub for SSL/TLS termination, request forwarding, and high-performance static asset caching.

#### Sample Nginx Virtual Host Config (`/etc/nginx/conf.d/gatherhub.conf`)

```nginx
server {
    listen 80;
    server_name event.domain.com;
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl http2;
    server_name event.domain.com;

    ssl_certificate /etc/letsencrypt/live/event.domain.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/event.domain.com/privkey.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;

    # Static uploads: Serve payment proofs and event banners directly from NFS/Disk
    location /payments/ {
        alias /var/www/gatherhub/storage/payments/;
        expires 30d;
        access_log off;
    }

    location /events/ {
        alias /var/www/gatherhub/storage/events/;
        expires 30d;
        access_log off;
    }

    # API and Admin routes: Reverse proxy to Go backend
    location / {
        proxy_pass http://localhost:3000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # Enable buffering for file uploads
        client_max_body_size 12M;
        proxy_buffering on;
    }
}
```

### 4. NFS Shared Storage (Horizontal Scaling)

When scaling the backend horizontally across multiple nodes (e.g., in a high-availability Kubernetes cluster or multi-server Docker Swarm), you **must** use a shared filesystem for `STORAGE_PATH` so all replicas can read/write payment proofs and banners.

#### Configuring NFS in `/etc/fstab` (Linux Host)

Mount the NFS share on your server hosts:
```text
192.168.1.100:/shares/gatherhub-storage /mnt/gatherhub-storage nfs rsize=1048576,wsize=1048576,hard,timeo=600,retrans=2,noresvport 0 0
```

#### Mounting NFS via Docker Volume (Docker Compose)

Configure the NFS mount directly in your Compose file:
```yaml
volumes:
  nfs_storage:
    driver: local
    driver_opts:
      type: nfs
      o: addr=192.168.1.100,rw,nfsvers=4.1,rsize=1048576,wsize=1048576,hard,timeo=600,retrans=2,noresvport
      device: ":/shares/gatherhub-storage"

services:
  backend:
    volumes:
      - nfs_storage:/storage
```

---

## 🛠️ Tech Stack

| Layer       | Technology                          |
|-------------|-------------------------------------|
| Backend     | Go 1.24 + Fiber v2                  |
| ORM         | GORM v2                             |
| Database    | PostgreSQL 17                       |
| Frontend    | HTMX + TailwindCSS                  |
| Proxy       | Nginx Alpine                        |
| Container   | Docker + Docker Compose             |
| XLSX Export | excelize v2                         |

---

## 📄 License

MIT © GatherHub