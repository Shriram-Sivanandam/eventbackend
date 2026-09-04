# Spotlight — Backend

REST API for Spotlight, a community events platform that lets anyone host and discover local events. Built with Go, PostgreSQL, Firebase, and Cloudflare R2.

---

## Tech Stack

| Technology                 | Purpose                               |
| -------------------------- | ------------------------------------- |
| Go + Chi                   | REST API framework                    |
| PostgreSQL                 | Primary database                      |
| Firebase Auth              | Custom token generation for chat      |
| Firebase Realtime Database | Real-time event chat                  |
| Firebase FCM               | Push notifications                    |
| Cloudflare R2              | Image storage (avatars, event photos) |
| Resend                     | OTP email delivery                    |
| Docker                     | Containerization                      |
| Railway                    | Deployment (staging + production)     |
| golang-migrate             | Database migrations                   |

---

## Architecture

```
React Native App
       ↓
   Chi Router
       ↓
  ┌────────────────────────────────────┐
  │  Handlers → DB Layer → PostgreSQL  │
  │  Handlers → Firebase FCM           │
  │  Handlers → Cloudflare R2          │
  │  Handlers → Firebase Auth          │
  └────────────────────────────────────┘
```

---

## Features

- **Email OTP authentication** — passwordless login via Resend
- **Events CRUD** — create, read, update, delete events with image upload to R2
- **Registration management** — hosts accept/reject attendees
- **Two-way ratings** — hosts rate attendees, attendees rate hosts
- **Real-time chat** — Firebase Realtime Database per event room
- **Push notifications** — FCM for registrations, reminders, chat
- **Event reminders** — background scheduler sends reminders 1 hour before events
- **Account deletion** — GDPR-compliant anonymization preserving rating integrity

---

## Project Structure

```
eventbackend/
├── cmd/
│   └── api/
│       └── main.go              # Entry point
├── internal/
│   ├── auth/                    # JWT creation and validation
│   ├── db/                      # Database query functions
│   ├── http/
│   │   ├── handlers/            # HTTP request handlers
│   │   └── middleware/          # Auth middleware
│   ├── notify/                  # FCM notification helpers
│   ├── scheduler/               # Background job (event reminders)
│   └── storage/                 # Cloudflare R2 client
├── db/
│   └── migrations/              # SQL migration files
├── Dockerfile
├── docker-compose.yml
└── go.mod
```

---

## Getting Started

### Prerequisites

- Go 1.25+
- Docker and Docker Compose
- A Firebase project with Auth and Realtime Database enabled
- A Cloudflare R2 bucket
- A Resend account with a verified domain

### Local Development

1. **Clone the repository**

```bash
git clone https://github.com/yourusername/eventbackend.git
cd eventbackend
```

2. **Create `.env.docker`**

```env
DATABASE_URL=postgres://postgres:password@db:5432/spotlight?sslmode=disable
JWT_SECRET=your_jwt_secret
FCM_PROJECT_ID=your_firebase_project_id
GOOGLE_APPLICATION_CREDENTIALS_JSON={"type":"service_account",...}
R2_ACCOUNT_ID=your_cloudflare_account_id
R2_ACCESS_KEY=your_r2_access_key
R2_SECRET_KEY=your_r2_secret_key
R2_BUCKET=your_bucket_name
R2_PUBLIC_URL=https://pub-xxx.r2.dev
```

3. **Start with Docker Compose**

```bash
docker-compose up --build
```

The server starts on `http://localhost:8080` and migrations run automatically on startup.

### Without Docker

```bash
# Install dependencies
go mod download

# Set environment variables (copy .env.docker to .env and update DATABASE_URL)
cp .env.docker .env

# Run
go run cmd/api/main.go
```

---

## API Documentation

Interactive API documentation available via Swagger UI:

**Staging:** https://eventbackend-staging.up.railway.app/swagger/index.html

---

## Database Schema

Key tables:

- `users` — user accounts with profile data and FCM tokens
- `events` — event listings with location, capacity, pricing
- `event_registrations` — registrations with status (pending/accepted/rejected)
- `event_ratings` — two-way ratings between hosts and attendees
- `event_messages` — chat messages (soft deletable)
- `tags` / `event_tags` — event categorisation
- `auth_otps` — OTP hashes with expiry

---

## Environment Variables

| Variable                              | Description                              |
| ------------------------------------- | ---------------------------------------- |
| `DATABASE_URL`                        | PostgreSQL connection string             |
| `JWT_SECRET`                          | Secret key for JWT signing               |
| `FCM_PROJECT_ID`                      | Firebase project ID                      |
| `GOOGLE_APPLICATION_CREDENTIALS_JSON` | Firebase service account JSON (minified) |
| `R2_ACCOUNT_ID`                       | Cloudflare R2 account ID                 |
| `R2_ACCESS_KEY`                       | Cloudflare R2 access key                 |
| `R2_SECRET_KEY`                       | Cloudflare R2 secret key                 |
| `R2_BUCKET`                           | R2 bucket name                           |
| `R2_PUBLIC_URL`                       | Public URL for R2 bucket                 |
| `RESEND_API_KEY`                      | Resend API key for email delivery        |

---

## Deployment

The backend is deployed on **Railway** with separate staging and production environments. Migrations run automatically on startup via `golang-migrate`.

- **Staging:** `https://eventbackend-staging.up.railway.app`
- **Production:** `https://api.spotlightinfo.in`

---

## License

MIT
