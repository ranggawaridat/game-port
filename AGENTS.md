# AGENTS.md

GamePort is a catalog of native Android game ports: a Go backend, PostgreSQL, and a
vanilla HTML/CSS/JS frontend with a built-in admin panel. Catalog content is entered
manually through the admin panel — there is no scraper or crawler.

## Layout (trust this over the README)

- `backend/` is the Go module root (`go.mod`, `cmd/server`, `internal/...`, `db/seed.sql`).
  Run all Go commands from here; the repo root has no module.
- `frontend/` is the entire UI, served by the backend as `STATIC_DIR`. No Node, bundler,
  package manager, or build step.
- `backend/db/seed.sql` is both the schema and the idempotent migration.
- `deploy/`, `Caddyfile*`, `gameport.service`, `docker-compose.yml`, `Dockerfile` are
  production ops, not local dev tooling.
- The root `README.md` still describes an older layout: it says `static/` and root-level
  Go commands. The actual paths are `frontend/` and `backend/`.

## Commands

From `backend/`:

```bash
go vet ./...
go build ./...
go test ./...        # no test files currently exist
go run ./cmd/server
```

Full stack from the repo root: `cp .env.example .env`, set `GAMEDB_PASSWORD` and
`ADMIN_PASSWORD`, then `docker compose up -d --build` (serves on `127.0.0.1:8181`).

Running the Go server directly (needs a reachable PostgreSQL):

```bash
cd backend
STATIC_DIR=../frontend UPLOAD_DIR=../uploads \
DATABASE_URL='postgres://gameport:pass@127.0.0.1:5432/gamedb?sslmode=disable' \
ADMIN_PASSWORD=dev go run ./cmd/server
```

- Env comes only from the process environment; there is no dotenv loader. `.env` is read
  by Docker Compose, systemd, and `deploy/manager.sh` — not by Go.
- `ADMIN_PASSWORD` (or `ADMIN_PASSWORD_HASH`) is mandatory; startup fails without it.
- `STATIC_DIR` defaults to `static` and `UPLOAD_DIR` to `uploads`, both relative to the
  working directory. From `backend/`, the UI is `../frontend` and uploads are `../uploads`.
- The Compose `db` service does not publish a host port, and the backend image bakes in
  `frontend/`. For UI changes run `docker compose up -d --build`; only `uploads/` is
  bind-mounted. For host-side `go run`, point `DATABASE_URL` at your own Postgres — the
  default `GAMEDB_HOST=db` only resolves inside Compose.

## Backend conventions

- Public routes are declared once in the `publik` slice in `internal/api/server.go` and
  mounted under both `/api/v1` (current) and `/api` (legacy migration alias). Add new
  endpoints there, not directly to the mux. Safelink routes live at `/dl/...`; admin
  routes live under `cfg.AdminPath`.
- Admin auth: `wajibAdmin` returns **404**, not 401, for unauthenticated non-HTML requests,
  and redirects HTML/HX requests to `ADMIN_PATH`. This hiding is intentional. The admin
  path is secret and never linked from the public site.
- The static handler serves `STATIC_DIR`, falls back to `index.html` for extensionless
  paths (so `/port/{slug}` works as a client route), and refuses to serve `ADMIN_PATH`.
- CORS applies only to paths starting with `/api/` and is driven by `CORS_ORIGINS`.
- Uploads are capped at 5 MB and validated by magic bytes, not filename extension; they
  are served from `/uploads/{file}`.
- The frontend and API are split in production: `app.js` hardcodes `gameport.web.id` to
  choose relative URLs vs `https://api.gameport.web.id`. Edit those constants if domains
  change, and keep using `asetAPI()` for database-stored relative image paths.
- `/port/{slug}` is a frontend route (Vercel rewrite in `frontend/vercel.json`), not an API
  route. Vercel Root Directory must be `frontend`.

## Database

- `backend/db/seed.sql` is the single source of schema. Compose runs it only when the data
  volume is empty; `deploy/deploy.sh` reapplies it on every deploy, so it must stay
  idempotent.
- For schema changes, add idempotent statements (`ALTER TABLE ... ADD COLUMN IF NOT EXISTS`,
  `CREATE TABLE/INDEX IF NOT EXISTS`) and keep reference seeds using `ON CONFLICT`. Editing
  only an initial `CREATE TABLE` will not migrate existing databases.
- `v_port_detail` (flattened joins/platform array) and `v_statistik` (homepage aggregates)
  are dropped and recreated on every seed run. Prefer querying them over re-joining tables.

## Deploy and ops (do not run from a dev checkout)

- `deploy/deploy.sh` and `deploy/manager.sh` hardcode `/opt/data/...` host/container paths
  and stop production. `manager.sh` runs inside the Hermes container (Postgres over a unix
  socket, app on `0.0.0.0:8181`). `deploy.sh` builds linux/amd64, stops the supervisor,
  copies `frontend/*` to the deployed `static/`, applies the seed, then health-checks.
- Caddy variants: `Caddyfile.domain` is the current three-subdomain setup (apex on Vercel,
  `api.` and `vps.` proxied; it hardcodes container IP `172.18.0.3`), `Caddyfile` is a
  single-domain template, `Caddyfile.ip` serves raw HTTP on `:80`. Never use a bare IP as a
  Caddy site address — Caddy forces HTTPS with a 308.
- `.env` values containing `&` or `?` must be quoted when sourced. `systemctl reload` does
  not reread `.env` (use `restart`), and a running binary cannot be overwritten.
- Deploy guides: `README-DEPLOY.md` (systemd/host) and `SETUP-DOMAIN.md` (DNS/HTTPS).

## Style

- Comments, UI strings, and many identifiers are Indonesian (`wajibAdmin`, `AmbilSetting`,
  `tampilLogin`). Match the surrounding style instead of translating.
- Commits use Conventional Commits (`feat(api): ...`, `fix(publik): ...`). There is no CI
  or linter config; verify with `go vet ./...`, `go build ./...`, and the health/curl
  checks in `deploy/deploy.sh` when a change affects serving.
- Never commit secrets; only `.env.example` is tracked. `.gitignore` excludes `.env*`,
  `uploads/*`, logs, and local DB/state files. Schema changes belong in
  `backend/db/seed.sql`.
