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
- **File Upload Support** — Payment proof uploads stored in persistent volume
- **Health Monitoring** — Built-in `/health` endpoint with DB ping validation
- **Docker-First** — Full containerized setup with PostgreSQL, Go backend, and Nginx reverse proxy
- **Auto Migration** — GORM auto-migrates schema on startup, no manual SQL needed

---

## 🗂️ Folder Structure

```
gatherhub/
├── backend/
│   ├── cmd/server/         # Application entrypoint (main.go)
│   ├── internal/
│   │   ├── config/         # Environment variable loader
│   │   ├── database/       # GORM connection + auto-migration
│   │   ├── handlers/       # HTTP handlers (health, etc.)
│   │   ├── middleware/      # Custom Fiber middleware
│   │   ├── models/         # GORM models (Event, Participant)
│   │   ├── routes/         # Route registration
│   │   └── services/       # Business logic layer
│   ├── migrations/         # Raw SQL migration files
│   ├── uploads/            # Local upload directory (dev only)
│   ├── Dockerfile          # Multi-stage Docker build
│   └── go.mod
├── frontend/
│   └── index.html          # HTMX + TailwindCSS landing page
├── nginx/
│   ├── nginx.conf          # Main Nginx config
│   └── conf.d/default.conf # Virtual host — API proxy + static serving
├── storage/
│   └── uploads/            # Persistent upload volume
├── docs/                   # API documentation (future)
├── docker-compose.yml      # Orchestration: postgres + backend + nginx
├── .env.example            # Environment variable template
└── README.md
```

---

## 🚀 Local Development (Without Docker)

### Prerequisites

- Go 1.23+
- PostgreSQL 17 running locally

### 1. Clone & configure

```bash
git clone <repository-url>
cd gatherhub

# Copy and edit environment variables
cp .env.example backend/.env
```

### 2. Start PostgreSQL

Ensure PostgreSQL is running with:
```
DB: gatherhub  |  User: gatherhub  |  Password: gatherhub
```

### 3. Run the backend

```bash
cd backend
go mod download
go run cmd/server/main.go
```

The server will start on **http://localhost:3000**

### 4. Verify it's working

```bash
curl http://localhost:3000
# → {"message":"GatherHub API Running"}

curl http://localhost:3000/health
# → {"status":"ok"}
```

### 5. Open the frontend

Open `frontend/index.html` in your browser directly, or serve it with:

```bash
npx serve frontend/
```

---

## 🐳 Docker Usage

### Start all services

```bash
docker compose up -d
```

This will start:
| Service   | Port | Description              |
|-----------|------|--------------------------|
| postgres  | 5432 | PostgreSQL 17 database   |
| backend   | 3000 | Go + Fiber API server    |
| nginx     | 80   | Reverse proxy + frontend |

### View logs

```bash
docker compose logs -f backend
docker compose logs -f postgres
```

### Stop all services

```bash
docker compose down
```

### Reset database (⚠️ deletes all data)

```bash
docker compose down -v
docker compose up -d
```

---

## 🔌 API Endpoints

| Method | Path      | Description           |
|--------|-----------|-----------------------|
| GET    | `/`       | API status message    |
| GET    | `/health` | Health check + DB ping |

### Responses

**GET /**
```json
{ "message": "GatherHub API Running" }
```

**GET /health**
```json
{ "status": "ok" }
```

---

## 📦 Models

### Event

| Field                 | Type      | Description               |
|-----------------------|-----------|---------------------------|
| id                    | uint      | Primary key               |
| title                 | string    | Event title               |
| description           | text      | Full description          |
| event_date            | timestamp | When the event occurs     |
| location              | string    | Event venue               |
| price                 | float64   | Registration fee          |
| payment_bank          | string    | Bank name                 |
| payment_account_number| string    | Bank account number       |
| payment_account_name  | string    | Account holder name       |
| admin_name            | string    | Organizer name            |
| admin_whatsapp        | string    | WhatsApp contact          |

### Participant

| Field              | Type   | Description                         |
|--------------------|--------|-------------------------------------|
| id                 | uint   | Primary key                         |
| event_id           | uint   | FK → Event                          |
| full_name          | string | Participant's full name             |
| phone              | string | Phone number                        |
| email              | string | Email address                       |
| city               | string | City of residence                   |
| company_name       | string | Company name                        |
| industrial_estate  | string | Industrial estate / area            |
| telegram_username  | string | Telegram handle                     |
| job_title          | *string| Job title (nullable)                |
| payment_proof      | string | File path to uploaded payment proof |
| status             | enum   | PENDING / VERIFIED / REJECTED       |

---

## ⚙️ Environment Variables

| Variable      | Default       | Description            |
|---------------|---------------|------------------------|
| APP_PORT      | 3000          | Server port            |
| APP_ENV       | development   | Environment name       |
| DB_HOST       | localhost     | Database host          |
| DB_PORT       | 5432          | Database port          |
| DB_USER       | gatherhub     | Database username      |
| DB_PASSWORD   | gatherhub     | Database password      |
| DB_NAME       | gatherhub     | Database name          |
| DB_SSLMODE    | disable       | PostgreSQL SSL mode    |
| UPLOAD_DIR    | ./uploads     | Upload storage path    |

---

## 🛠️ Tech Stack

| Layer       | Technology                          |
|-------------|-------------------------------------|
| Backend     | Go 1.23 + Fiber v2                  |
| ORM         | GORM v2                             |
| Database    | PostgreSQL 17                       |
| Frontend    | HTMX + TailwindCSS                  |
| Proxy       | Nginx Alpine                        |
| Container   | Docker + Docker Compose             |

---

## 📄 License

MIT © GatherHub