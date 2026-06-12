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
└── gatherhub-storage/          ← Runtime uploads (OUTSIDE the repo)
    ├── payments/               # Payment proof uploads
    ├── events/                 # Event banner uploads
    └── temp/                   # Temporary files
```

> **Important:** `gatherhub-storage/` is a sibling of the `gatherhub/` repository directory. It is **never** committed to Git.

---

## 📦 Storage Configuration

All runtime uploads are stored outside the Git repository using a single environment variable:

```
STORAGE_PATH=/absolute/path/to/gatherhub-storage
```

Sub-directories are created automatically on startup:

| Path | Purpose |
|------|---------|
| `{STORAGE_PATH}/payments/` | Payment proof uploads (jpg, png, pdf) |
| `{STORAGE_PATH}/events/` | Event banner images |
| `{STORAGE_PATH}/temp/` | Temporary/scratch files |

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

### 1. Create the storage directory

```bash
# Sibling of the repo directory
mkdir -p ../gatherhub-storage/{payments,events,temp}
```

### 2. Start all services

```bash
docker compose up -d
```

The compose file mounts `../gatherhub-storage` as `/storage` inside the backend container and `STORAGE_PATH=/storage` is set automatically.

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
| `STORAGE_PATH`   | `../gatherhub-storage` | **Root path for all runtime uploads**      |
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