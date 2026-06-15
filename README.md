# SWE CloudBuild

Local web application for building SWE-bench Docker images through Volcengine cloud services.

## Foundation quickstart

1. Copy `.env.example` to `.env` and fill local values as needed.
2. Start PostgreSQL:
   ```bash
   docker compose up -d postgres
   ```
3. Run the backend:
   ```bash
   cd server
   go run ./cmd/server
   ```
4. Run the frontend after installing JS dependencies:
   ```bash
   cd web
   npm install
   npm run dev
   ```

The first runnable demo targets mock mode by default and uses PostgreSQL from `arm64v8/postgres:15`.
