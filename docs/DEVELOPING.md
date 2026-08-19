# GoBook

A REST API built with [Gin](https://gin-gonic.com) and [GORM](https://gorm.io),
laid out in an MVC structure with a **service layer** and a **repository layer**
between the controller and the database.

It ships with one worked-through resource (`User`), versioned migrations with a
seeding CLI, and a code generator that scaffolds any further resource across
every layer.

**Contents** — [Requirements](#requirements) · [Install](#install) ·
[Quick start](#quick-start) ·
[Commands](#commands) · [Configuration](#configuration) ·
[Project structure](#project-structure) · [Architecture](#architecture) ·
[API](#api) · [Migrations](#migrations-and-seeding) ·
[Code generation](#code-generation) · [Renaming](#renaming-the-project) ·
[Authentication](#authentication) · [Rate limiting](#rate-limiting) ·
[Testing](#testing) ·
[Limitations](#limitations-and-next-steps)

---

## Requirements

| | |
| --- | --- |
| Go | 1.25+ — pinned by the `go` directive in `go.mod`; lower it there if you need an older toolchain |
| MySQL | 5.7+ / 8.x |
| make | optional — every target is a plain `go` command you can run directly |

---

## Install

This is an application, not a library — there is nothing to `go get`. Clone it
and build from the repository root.

```bash
git clone https://github.com/scriptertoufiq/gobook.git
cd gobook
make setup          # copies .env.example -> .env, downloads modules
```

Then follow [Quick start](#quick-start) from step 2.

### As a starter for your own project

The generator can rewrite the module path, so the usual move is to take a copy
with no shared history rather than a fork:

```bash
git clone --depth 1 https://github.com/scriptertoufiq/gobook.git shop
cd shop
rm -rf .git && git init          # detach from this repository's history

go run ./cmd/make rename github.com/you/shop -app-name "Shop API"
go mod tidy
make setup

git add -A && git commit -m "initial commit"
git remote add origin git@github.com:you/shop.git
git push -u origin HEAD          # HEAD, so it works on master or main
```

`rename` re-parses every `.go` file after substitution, so a rewrite that would
break the build is rejected before anything is written. It does **not** rename
the directory or touch a git remote — both are done by hand above. Full detail
in [Renaming the project](#renaming-the-project).

> Prefer GitHub's **Use this template** / **Fork** button instead if you want
> the link back to this repository kept. The `rename` step is the same either
> way.

---

## Quick start

```bash
# 1. config + dependencies
make setup                     # copies .env.example -> .env, downloads modules

# 2. create the database (the app creates tables, not the schema itself)
mysql -u root -p -e "CREATE DATABASE go_mvc CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"

# 3. in .env, set your DB credentials and a signing key
#    DB_USER=root
#    DB_PASSWORD=...
#    JWT_SECRET=$(openssl rand -base64 48)   # required — boot fails without it

# 4. build the tables and insert demo rows
make migrate-fresh

# 5. run
make run                       # http://localhost:8080
```

Verify it:

```bash
curl localhost:8080/health

# /users needs a token now — log in as the seeded admin first
curl -X POST localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@example.com","password":"password"}'
curl localhost:8080/api/v1/users -H "Authorization: Bearer <access_token>"
```

Seeded accounts (password `password` for both, both pre-verified):

| Email | Role |
| --- | --- |
| `admin@example.com` | admin |
| `jane@example.com` | user |

---

## Commands

Every `make` target is a thin wrapper — the right-hand column is what actually
runs, and works identically if you'd rather skip make.

### Everyday

| Command | Runs | What it does |
| --- | --- | --- |
| `make setup` | `cp .env.example .env` + `go mod download` | One-time setup |
| `make run` | `go run ./cmd/api` | Starts the server on `APP_PORT` (default 8080) |
| `make test` | `go test ./... -race -count=1` | Runs the test suite |
| `make build` | `go build -o bin/...` | Compiles `bin/api` and `bin/migrate` for deploy |
| `make clean` | `rm -rf bin` | Removes build output |

### Database

| Command | Runs | What it does |
| --- | --- | --- |
| `make migrate` | `go run ./cmd/migrate` | Applies every pending migration |
| `make migrate-status` | `go run ./cmd/migrate -status` | Lists what has run and what hasn't |
| `make migration name=…` | `go run ./cmd/make migration …` | Writes a new migration file |
| `make migrate-rollback` | `go run ./cmd/migrate -rollback` | Reverses the last batch. `steps=N` for more |
| `make seed` | `go run ./cmd/migrate -seed` | Inserts demo rows. Idempotent |
| `make migrate-fresh` | `go run ./cmd/migrate -fresh -seed` | **Drops every table**, replays, seeds |

`-fresh` refuses to run when `APP_ENV=production`, with no override.
`-rollback` is refused there too unless you pass `-force`.

`-seed` is **development-only**. The fixtures include an admin account whose
password is published in this README, so seeding is gated on an allow-list —
`local`, `development`, `dev`, `test`, `testing`. An unfamiliar `APP_ENV` such
as `staging`, or a typo, fails closed rather than seeding a public admin.

### Code generation

| Command | Runs | What it does |
| --- | --- | --- |
| `make scaffold name=Category` | `go run ./cmd/make scaffold Category` | Generates a full resource + wiring |
| `make rename module=... app=...` | `go run ./cmd/make rename ...` | Renames the module path and/or `APP_NAME` |

Full generator reference: `go run ./cmd/make -h`

| Sub-command | Generates | Also wires |
| --- | --- | --- |
| `scaffold` | all eight files below | container + routes |
| `model` | `internal/models/<name>.go` | + a create-table migration |
| `migration` | `internal/database/migrations/<timestamp>_<name>.go` | — (self-registering) |
| `repository` | `internal/repositories/<name>_repository.go` | container |
| `service` | `internal/services/<name>_service.go` | container |
| `controller` | `internal/controllers/<name>_controller.go` | container + routes |
| `request` | `internal/requests/<name>_request.go` | — |
| `resource` | `internal/resources/<name>_resource.go` | — |
| `test` | `internal/services/<name>_service_test.go` | — |
| `rename` | — | module path across all files, `APP_NAME` |

Flags: `-force` (overwrite existing files), `-no-wire` (generate only),
`-app-name` and `-dry-run` (rename only). Flags may appear in any position
after the sub-command.

### Housekeeping

| Command | Runs |
| --- | --- |
| `make fmt` | `go fmt ./...` |
| `make vet` | `go vet ./...` |
| `make tidy` | `go mod tidy` |

### Direct binaries

`go run ./cmd/X` compiles the `package main` in that directory and runs it —
nothing is installed. Three programs live under `cmd/`:

```bash
go run ./cmd/api                       # the web server
go run ./cmd/migrate [-status|-rollback|-fresh] [-seed]   # schema + seed data
go run ./cmd/make <sub-command> ...    # code generator
```

> **Note on the name:** `make run` is GNU make reading the `Makefile`.
> `go run ./cmd/make` is *this project's generator*, named after
> `php artisan make:model`. They are unrelated — rename `cmd/make/` to
> `cmd/gen/` if the collision bothers you; only the `scaffold` and `rename`
> Makefile targets reference it.

---

## Configuration

All configuration is environment-driven. `config/config.go` is the only file
that reads `os.Getenv`; everything else receives a `*config.Config`.

`.env` is gitignored — `.env.example` is the template.

| Variable | Default | Purpose |
| --- | --- | --- |
| `APP_NAME` | `GoBook` | Label in the startup log and `/health` |
| `APP_ENV` | `production` | **Unset defaults to production**, so a forgotten variable fails safe. `.env.example` sets `local` |
| `APP_URL` | `http://localhost:8080` | Base for verification links |
| `APP_PORT` | `8080` | HTTP listen port |
| `APP_DEBUG` | *(dev only)* | Verbose SQL logging; echoes panic detail. Defaults on only when `APP_ENV` is a development value |
| `APP_SHUTDOWN_TIMEOUT` | `10` | Seconds to drain in-flight requests on SIGTERM |
| `DB_HOST` | `127.0.0.1` | |
| `DB_PORT` | `3306` | |
| `DB_USER` | `root` | |
| `DB_PASSWORD` | *(empty)* | |
| `DB_NAME` | `go_mvc` | Must exist before the first migration |
| `DB_CHARSET` | `utf8mb4` | |
| `DB_MAX_IDLE_CONNS` | `10` | Pool tuning |
| `DB_MAX_OPEN_CONNS` | `100` | Pool tuning |
| `DB_CONN_MAX_LIFETIME` | `60` | Minutes |
| `CORS_ALLOWED_ORIGINS` | *(empty = any)* | Comma-separated list. **Tighten before production** |
| `RATE_LIMIT_ENABLED` | `true` | Master switch |
| `RATE_LIMIT_REQUESTS` | `120` | General allowance per window |
| `RATE_LIMIT_WINDOW` | `60` | General window, seconds |
| `RATE_LIMIT_AUTH_REQUESTS` | `10` | Allowance for `/auth/*` |
| `RATE_LIMIT_AUTH_WINDOW` | `60` | Auth window, seconds |
| `TRUSTED_PROXIES` | *(empty)* | CIDRs whose `X-Forwarded-For` is believed. **Security-relevant** |
| `JWT_SECRET` | *(none)* | **Required**, min 32 chars. Boot fails without it |
| `JWT_ISSUER` | `gobook` | `iss` claim |
| `JWT_ACCESS_TTL` | `15` | Access token lifetime, minutes |
| `JWT_REFRESH_TTL` | `720` | Refresh token lifetime, hours |
| `AUTH_REQUIRE_EMAIL_VERIFICATION` | `false` | The verification switch |
| `AUTH_VERIFICATION_TTL` | `24` | Verification link lifetime, hours |
| `AUTH_PASSWORD_RESET_TTL` | `60` | Reset link lifetime, **minutes** |
| `AUTH_PASSWORD_RESET_URL` | `{APP_URL}/reset-password` | Frontend form the reset link points at |
| `MAIL_HOST` / `MAIL_PORT` | `127.0.0.1` / `1025` | SMTP relay |
| `MAIL_USERNAME` / `MAIL_PASSWORD` | *(empty)* | Leave empty for a local relay |
| `MAIL_FROM_ADDRESS` / `MAIL_FROM_NAME` | *(empty)* / `GoBook` | Sender identity |

A bad DSN fails at boot with a clear error rather than on the first request —
`database.Connect` pings the pool before returning.

---

## Project structure

```
cmd/                      entrypoints — one directory per binary
  api/                    HTTP server
  migrate/                migration + seeding CLI
  make/                   code generator (scaffold, rename)

config/                   env loading; the only place reading os.Getenv

internal/                 application code — unimportable from outside
  app/                    bootstrap: config -> db -> container -> router -> server
  container/              composition root; wires repos -> services -> controllers
  controllers/            HTTP edge — the "C"
  database/               connection + the migration runner
    migrations/           versioned schema history, one file per change
    seeders/              demo/baseline rows
  middleware/             auth/roles/verification, request id, logging, recovery, CORS
  models/                 GORM schemas + domain entities — the "M"
  repositories/           all query building; the only packages touching *gorm.DB
  requests/               input DTOs with validation tags
  resources/              output DTOs — the "V" of a JSON API
  routes/                 the routing table
  services/               business rules; no HTTP, no Gin

pkg/                      generic helpers, safe to extract into another project
  apperror/               transport-agnostic errors with HTTP status mapping
  hash/                   bcrypt wrapper for passwords
  jwt/                    access-token signing and validation
  mailer/                 Mailer interface + SMTP implementation
  ratelimit/              per-key token bucket used by the throttle
  token/                  random opaque tokens (refresh, verification, reset)
  pagination/             query parsing + page metadata
  response/               the single JSON envelope every endpoint uses
```

`internal/` is enforced by the Go toolchain — no other module can import it.
That boundary is why business logic lives there and only reusable plumbing sits
in `pkg/`.

### Which file do I edit?

| I want to… | File |
| --- | --- |
| Add/change a database column | `internal/models/*.go`, then `make migration name=…` and `make migrate` |
| Change a query or add a filter | `internal/repositories/*_repository.go` |
| Change a business rule | `internal/services/*_service.go` |
| Change validation rules | `internal/requests/*_request.go` |
| Change what JSON comes back | `internal/resources/*_resource.go` |
| Add or change a URL | `internal/routes/api.go` |
| Add a cross-cutting concern | `internal/middleware/` |
| Change config | `.env`, and `config/config.go` for a new key |

---

## Architecture

### Request flow

```
HTTP request
  ├─ middleware        request id → logging → panic recovery → CORS
  ├─ routes            match method + path
  ├─ controller        bind requests.XRequest, validate  ─── 422 on failure
  ├─ service           business rules                    ─── 404 / 409 / 401 …
  ├─ repository        GORM query
  └─ model             row ↔ struct
       ↑
     resource          model → public JSON shape
     response          uniform envelope
```

### The rule

**Each layer only talks to the one below it.** A controller never touches
`*gorm.DB`; a service never imports Gin. Three consequences worth the discipline:

1. **The service layer is unit-testable without a database.** Services depend on
   a repository *interface*, so tests swap in an in-memory fake — see
   `internal/services/user_service_test.go`.
2. **Columns can't leak.** Handlers bind into `requests.*` DTOs and return
   `resources.*` DTOs, so adding a `password` column can't accidentally
   serialise it, and a client can't mass-assign `role` or `id`.
3. **Errors carry their own status.** Services return `*apperror.Error`; the
   controller just calls `response.Error(c, err)` and the right status comes out.

### Boot sequence

`cmd/api/main.go` → `app.New()`:

```
config.Load()        read .env
database.Connect()   open pool + ping
gin.New() + middleware
container.Build()    repos → services → controllers
routes.Register()    mount the routing table
```

`app.Run()` then serves until SIGINT/SIGTERM, drains in-flight requests, and
closes the pool.

---

## API

Base path: `/api/v1`

### Response envelope

Every endpoint answers in the same shape.

**Success**

```json
{ "success": true, "data": { "id": 1, "name": "Admin" } }
```

**Paginated**

```json
{
  "success": true,
  "data": [],
  "meta": { "page": 1, "per_page": 15, "total": 42, "last_page": 3 }
}
```

**Error** — `fields` appears only on validation failures. Keys are the JSON
names the client sent, and each message is a complete sentence:

```json
{
  "success": false,
  "error": {
    "code": "validation_failed",
    "message": "3 fields need attention. See `error.fields` for details.",
    "fields": {
      "name": "Name is required.",
      "email": "Email must be a valid email address.",
      "password": "Password must be at least 8 characters."
    }
  }
}
```

When exactly one field fails, the top-level `message` restates it, so a simple
client can show `error.message` and ignore `error.fields` entirely.

**422 vs 400.** A body that parsed but broke a rule is `422` with `fields`. A
body that could not be parsed at all is `400` with a specific explanation —
empty, malformed, or the wrong type for a field. Conflating the two sends
developers hunting through field rules when the real answer is a missing brace:

```json
{ "success": false, "error": {
    "code": "bad_request",
    "message": "Request body is not valid JSON — it ends unexpectedly. Check for a missing brace or quote." } }
```

| Status | `code` | When |
| --- | --- | --- |
| 400 | `bad_request` | Malformed route parameter |
| 401 | `unauthorized` | Bad credentials |
| 403 | `forbidden` | Account disabled, or insufficient role |
| 403 | `email_not_verified` | Verification required but not completed |
| 404 | `not_found` | Unknown record or route |
| 405 | `method_not_allowed` | Wrong verb for a known path; see the `Allow` header |
| 409 | `conflict` | Duplicate email or slug |
| 422 | `validation_failed` | Request body failed binding rules |
| 429 | `rate_limited` | Throttled; see `Retry-After` |
| 500 | `internal_error` | Anything else |
| 503 | `database_unavailable` | `/health` could not ping MySQL |

Internal errors never leak their cause. `response.Error` attaches the real error
to the Gin context so the logging middleware records it, and returns a generic
message to the client.

### Endpoints

🔓 public · 🔒 needs a token · ✅ additionally needs a verified email (when the
feature is on) · 👑 admin only · 🙋 your own record, or admin

| | Method | Path | Description |
| --- | --- | --- | --- |
| 🔓 | GET | `/health` | Readiness probe — pings the database |
| 🔓 | POST | `/api/v1/auth/register` | Create an account |
| 🔓 | POST | `/api/v1/auth/login` | Exchange credentials for a token pair |
| 🔓 | POST | `/api/v1/auth/refresh` | Rotate the pair; the old refresh token dies |
| 🔓 | GET | `/api/v1/auth/verify-email?token=` | Target of the emailed link |
| 🔓 | POST | `/api/v1/auth/password/forgot` | Email a reset link |
| 🔓 | POST | `/api/v1/auth/password/reset` | Redeem the token, set a new password |
| 🔒 | GET | `/api/v1/auth/me` | The caller's own profile |
| 🔒 | POST | `/api/v1/auth/logout` | Revoke one session, or all with `{"all":true}` |
| 🔒 | POST | `/api/v1/auth/email/resend` | New verification link for the caller |
| 🔒 | POST | `/api/v1/auth/password/change` | Change your own password |
| 🔒✅👑 | GET | `/api/v1/users` | Paginated list |
| 🔒✅👑 | POST | `/api/v1/users` | Create |
| 🔒✅🙋 | GET | `/api/v1/users/:id` | Show |
| 🔒✅🙋 | PATCH · PUT | `/api/v1/users/:id` | Partial update. Password changes to **your own** record go to `/auth/password/change` |
| 🔒✅👑 | DELETE | `/api/v1/users/:id` | Soft delete |

`User` is the only business resource in the box — the worked example of the
layering. Add your own with [the generator](#code-generation).

> **Opening an endpoint in a browser?** The address bar always sends `GET`.
> Every endpoint that takes a body is `POST`-only, so you'll get a 405 with an
> `Allow` header listing what it does accept. Only `GET /health` and
> `GET /api/v1/auth/verify-email?token=` are browsable; use curl, Postman, or
> `fetch()` for the rest.

### Query parameters

Available on every list endpoint:

| Param | Default | Notes |
| --- | --- | --- |
| `page` | `1` | |
| `per_page` | `15` | Capped at 100 |
| `search` | — | Matches name or email |
| `sort_by` | `id` | Whitelisted per resource; anything else falls back to `id` |
| `sort_dir` | `desc` | `asc` or `desc` |

Sortable columns for users: `id`, `name`, `email`, `created_at` — declared as
`userSortable` in `internal/repositories/user_repository.go`. The whitelist is
why `sort_by` never reaches the SQL builder verbatim.

### Examples

```bash
# list, paginated + searched
curl "localhost:8080/api/v1/users?page=1&per_page=10&search=admin&sort_by=name&sort_dir=asc"

# create
curl -X POST localhost:8080/api/v1/users \
  -H 'Content-Type: application/json' \
  -d '{"name":"Toufiq","email":"t@example.com","password":"supersecret"}'

# partial update — only the supplied fields change
curl -X PATCH localhost:8080/api/v1/users/1 \
  -H 'Content-Type: application/json' \
  -d '{"name":"Renamed"}'

# login
curl -X POST localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@example.com","password":"password"}'

# soft delete
curl -X DELETE localhost:8080/api/v1/users/1
```

---

## Authentication

JWT with **access + refresh tokens**, plus email verification you can switch on
per environment.

### Two settings control everything

```ini
JWT_SECRET=                        # REQUIRED — the app refuses to boot without it
AUTH_REQUIRE_EMAIL_VERIFICATION=false
```

`JWT_SECRET` must be at least 32 characters. There is deliberately no default:
a signing key baked into source lets anyone who reads the repo mint valid
tokens, so `config.Validate()` fails the boot instead. Generate one with
`openssl rand -base64 48`.

`AUTH_REQUIRE_EMAIL_VERIFICATION` is the feature switch. Off, the whole flow is
inert — no mail is sent, no route is gated, and the same route table serves both
configurations. On, `MAIL_HOST` and `MAIL_FROM_ADDRESS` become required.

### The flow

```bash
# 1. register
curl -X POST localhost:8080/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"name":"Toufiq","email":"t@example.com","password":"supersecret"}'

# 2. login -> access_token (15 min) + refresh_token (30 days)
curl -X POST localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"t@example.com","password":"supersecret"}'

# 3. call a protected route
curl localhost:8080/api/v1/users -H "Authorization: Bearer <access_token>"

# 4. when the access token expires, rotate
curl -X POST localhost:8080/api/v1/auth/refresh \
  -H 'Content-Type: application/json' \
  -d '{"refresh_token":"<refresh_token>"}'

# 5. logout — one session, or every session
curl -X POST localhost:8080/api/v1/auth/logout \
  -H "Authorization: Bearer <access_token>" \
  -H 'Content-Type: application/json' -d '{"all":true}'
```

### Design notes

**Refresh tokens rotate.** Every `/auth/refresh` revokes the token you presented
and issues a new one, so a stolen token is usable at most once — and the theft
surfaces when the legitimate holder's next refresh fails.

**Only hashes are stored.** Refresh and verification tokens are 256 bits of
`crypto/rand`, persisted as SHA-256. A database leak yields nothing presentable
to the API. SHA-256 rather than bcrypt is deliberate: these are already random,
so slowing an attacker buys nothing, and a deterministic hash is what makes
`WHERE token_hash = ?` an indexed lookup.

**Failures are indistinguishable.** Unknown, expired, revoked, and malformed
tokens all return the same 401. So do "wrong password" and "no such user".

**Unverified users can log in.** They reach `/auth/me` and
`/auth/email/resend` but get `403 email_not_verified` elsewhere. This keeps
resend behind the caller's own token instead of a public endpoint that would be
both an account-enumeration oracle and a spam relay.

**The verified flag lives in the JWT**, so gated routes cost no extra query. The
trade-off: a token minted before verification reads `verified: false` until it
expires (`JWT_ACCESS_TTL`, 15 min default). Calling `/auth/refresh` updates it
immediately.

**Changing a credential closes the sessions it opened.** `UserService.Update` is
the single choke point for password changes — the direct `PATCH /users/:id` path
and `/auth/password/reset` both route through it — and it revokes every refresh
token whenever the password changes or the account is disabled. Deleting an
account does the same. Two ways to change a password must not have two different
security outcomes.

**Changing your own password has its own endpoint**, `POST /auth/password/change`.
It always requires `current_password` — an access token is short-lived proof of
a past login, not proof the holder knows the credential — and always returns a
fresh pair, since the change revokes every session including the caller's.

This applies **regardless of role**. An admin setting somebody else's password
via `PATCH /users/:id` is administrative; an admin changing their *own* is a
self-service change and goes through the same endpoint as everyone else. They
are the highest-value account in the system, not the least protected.

Keeping password changes out of `PATCH /users/:id` also gives that endpoint a
single response shape. Returning a token payload from a PATCH that usually
returns a user is a contract no client can code against.

**Verification links are never logged.** `middleware.Logger` redacts `token`,
`password`, `secret` and similar query parameters, because logs get shipped and
read by people who should never hold a live credential.

**Changing your email clears the verified stamp** and revokes every session. A
new address hasn't been proved, so the old proof cannot carry over — otherwise
verifying a throwaway and then switching addresses would walk straight past
`RequireVerified`. Revoking sessions matters too: their access tokens still
carry `verified: true`. A fresh link goes to the new address automatically, and
every creation path — self-service registration and `POST /users` alike — sends
one, so no account is left unverified with no way to fix it.

**A password reset does not instantly kill access tokens.** Reset revokes every
refresh token, but access tokens already issued stay valid until they expire —
up to `JWT_ACCESS_TTL` (15 minutes by default). That is inherent to stateless
JWTs: verifying revocation on every request would mean a database read per
request, which is the cost the design avoids. Shorten `JWT_ACCESS_TTL` to
narrow the window, or add a `tokens_valid_after` column checked in `Auth` if
you need instant revocation.

**Registration can't grant a role.** `RegisterRequest` has no role field; the
service hardcodes `models.RoleUser`.

### Password reset

```bash
# 1. request a link — the response is identical whether or not the account exists
curl -X POST localhost:8080/api/v1/auth/password/forgot \
  -H 'Content-Type: application/json' -d '{"email":"t@example.com"}'

# 2. your frontend form at AUTH_PASSWORD_RESET_URL reads ?token= and posts it
curl -X POST localhost:8080/api/v1/auth/password/reset \
  -H 'Content-Type: application/json' \
  -d '{"token":"<from the link>","password":"my-new-password"}'
```

Unlike the verification link — which the API redeems itself with a `GET` — a
reset link points at **your frontend**, because the user has to type a new
password. Set `AUTH_PASSWORD_RESET_URL` to that page; it defaults to
`{APP_URL}/reset-password`.

Three things this flow does deliberately:

- **`/password/forgot` always returns the same 200.** Unknown address, disabled
  account, even an SMTP outage — all identical. It is unauthenticated, so any
  observable difference would let anyone test which emails have accounts.
  Failures are logged for you, not returned to the caller.
- **A successful reset revokes every refresh token.** People reset passwords
  *because* an account is compromised; leaving the attacker's sessions alive
  would defeat the point.
- **Links are single-use, and requesting a new one retires the old.** TTL is 60
  minutes by default — much shorter than a verification link, since a live reset
  link is a temporary key to the account.

### Authorising the record, not just the caller

Being signed in is never enough on its own to touch someone else's data. Two
guards sit alongside `Auth`:

| Guard | Meaning |
| --- | --- |
| `RequireRole("admin")` | caller holds the role |
| `RequireSelfOrRole("id", "admin")` | the `:id` is the caller's own, or they're an admin |

Both **fail closed** when `Auth` hasn't run, so mounting them in the wrong order
denies rather than grants — there are tests for exactly that.

Ownership alone still isn't sufficient for every field. `role` and `is_active`
stay administrative even on your own record, or self-edit would be a
self-service route to admin. `UserController.Update` rejects them with a 403
rather than dropping them silently, so a caller who thinks they changed their
role is told they did not.

User listing is admin-only: it returns every account's email address.

### Protecting your own routes

Scaffolded resources are **protected by default** — the generator emits
`authenticated, verified` on the group. Delete them to make a resource public.
Forgetting to add a guard is silent; deleting one is a deliberate act:

```go
products := api.Group("/products", authenticated, verified)
products.DELETE("/:id", middleware.RequireRole("admin"), c.Product.Destroy)
```

### Testing email locally

`net/smtp` refuses to send credentials over an unencrypted link, so leave
`MAIL_USERNAME` empty for a local relay:

```bash
docker run -d --name mailpit -p 1025:1025 -p 8025:8025 axllent/mailpit
# captured mail: http://localhost:8025
```

---

## Rate limiting

Two tiers, both configurable, both on by default:

```ini
RATE_LIMIT_ENABLED=true
RATE_LIMIT_REQUESTS=120       # general, per window
RATE_LIMIT_WINDOW=60          # seconds
RATE_LIMIT_AUTH_REQUESTS=10   # stricter tier for /api/v1/auth/*
RATE_LIMIT_AUTH_WINDOW=60
```

The general tier covers every `/api/v1` route. The auth tier stacks on top,
because those endpoints send mail and run bcrypt on behalf of callers who
haven't authenticated yet. `RATE_LIMIT_ENABLED=false` makes the middleware a
pass-through — the route table is unchanged either way.

Every response carries the budget, so a client can back off *before* it gets
rejected rather than after:

```
X-RateLimit-Limit: 10
X-RateLimit-Remaining: 7
X-RateLimit-Reset: 1786351316      # unix time when the bucket is full again
Retry-After: 3                     # on 429 only
```

A rejection is the standard envelope with `"code": "rate_limited"` and status
429.

### How callers are identified

By **client IP**, always. Throttling has to run before any expensive work, which
means it runs before authentication — so there is no user identity available to
key on yet. Everyone behind one NAT therefore shares a bucket.

To add a per-user tier, mount a second `RateLimit` *after* `Auth` on the
protected groups and key it from `middleware.CurrentUserID`.

### TRUSTED_PROXIES — read this before deploying

Gin trusts every proxy by default, meaning `X-Forwarded-For` decides the client
IP. Since anonymous callers are throttled by IP, that default would let anyone
rotate the header and mint unlimited fresh buckets — the throttle would be
decorative. `app.New` therefore calls `SetTrustedProxies` explicitly.

| Deployment | Setting |
| --- | --- |
| App directly exposed | leave **empty** — uses the socket address, header ignored |
| Behind a load balancer / ingress | list its CIDR, e.g. `10.0.0.0/8` |
| Anything | **never** `0.0.0.0/0` — that restores the spoofable default |

Get this wrong in the other direction and every caller shares one bucket,
because `ClientIP()` reports the balancer.

> Testing locally: `curl localhost` resolves to IPv6 `::1`, so
> `TRUSTED_PROXIES=127.0.0.1/32` will *not* match. Use `curl --ipv4
> http://127.0.0.1:8080`, or include `::1/128`.

### Implementation

`pkg/ratelimit` is a per-key token bucket — not a fixed window, which would let
a caller fire twice the limit across a boundary by spending window N's budget at
the last instant and N+1's at the first.

Idle buckets are evicted by a janitor goroutine. That is not housekeeping but
the memory bound: without it, one key per spoofed source address grows the map
forever, and the limiter becomes the denial-of-service vector it exists to
prevent. The janitors stop during graceful shutdown via `Container.Close()`.

---

## Migrations and seeding

Migrations are **versioned files** in `internal/database/migrations/`, applied
in order and recorded in a `migrations` ledger table so each one runs exactly
once.

```bash
go run ./cmd/migrate                      # apply everything pending
go run ./cmd/migrate -status              # what has run, what hasn't
go run ./cmd/migrate -seed                # migrate, then seed
go run ./cmd/migrate -rollback            # reverse the last batch
go run ./cmd/migrate -rollback -steps 3   # reverse the last three batches
go run ./cmd/migrate -fresh               # DROP every table, then replay
go run ./cmd/migrate -fresh -seed
```

`-status` prints the history:

```
  STATUS    BATCH  MIGRATION
  --------  -----  ---------
  applied   1      20260813000001_create_users_table
  applied   1      20260813000002_create_refresh_tokens_table
  applied   2      20260813152336_create_categories_table
  pending   -      20260813161500_add_status_to_posts
```

### Writing one

```bash
go run ./cmd/make migration add_status_to_posts
# → internal/database/migrations/20260813161500_add_status_to_posts.go
```

Each file registers itself from `init`, so **there is no list to maintain** —
dropping the file in is the whole of the installation, and two branches adding
migrations never conflict.

```go
func init() {
	Register(Migration{
		ID: "20260813161500_add_status_to_posts",

		Up: func(db *gorm.DB) error {
			return db.AutoMigrate(&models.Post{})
		},
		Down: func(db *gorm.DB) error {
			return db.Migrator().DropColumn(&models.Post{}, "status")
		},
	})
}
```

Migrations are written **against the models** wherever they can be, so the
struct stays the single description of the schema instead of every column being
duplicated as SQL. `AutoMigrate` is additive and idempotent, which is what makes
this safe to run against a database that already has the table.

For what `AutoMigrate` can't express, reach for the migrator or raw SQL:

```go
db.Migrator().DropColumn(&models.Post{}, "subtitle")
db.Migrator().RenameColumn(&models.Post{}, "body", "content")
db.Exec("UPDATE posts SET status = ? WHERE status = ''", "draft")
```

### Rules worth knowing

| | |
| --- | --- |
| **IDs are permanent** | The ID is the ledger key. Renaming one after it has run makes it run again |
| **Batches** | Everything applied in one run shares a batch, so `-rollback` undoes a *deploy*, not an arbitrary number of changes |
| **`Down: nil`** | Marks a migration irreversible. Rollback then refuses the whole batch rather than unwinding half of it |
| **No transaction** | MySQL commits DDL implicitly, so wrapping the run would only *look* atomic. Each migration is recorded the moment it succeeds, and a failure stops the run with the ledger accurate — re-running resumes at the one that broke |
| **Orphans** | A ledger row whose file is gone shows as `orphaned` in `-status`, and blocks a rollback that would need it |

### Adopting an existing database

The first run over a schema that already exists is safe: every migration's `Up`
is an `AutoMigrate` that finds nothing to do, and the ledger simply records
them as applied. No data is touched.

### Guard rails

`-fresh` refuses to run when `APP_ENV=production` and has no override —
rebuilding a production database from empty is not an operation this command
offers. `-rollback` is gated on the same check but accepts `-force`, since
reversing a bad deploy is a legitimate thing to need.

### Seeders

`internal/database/seeders/` holds baseline and demo rows. Seeders are
idempotent — they use `ON CONFLICT DO NOTHING` against the unique index, so
running them twice inserts nothing the second time. Register a new one in
`seeders.Run`.

### Token tables

Three tables back the auth flows, all storing SHA-256 hashes rather than the
tokens themselves:

| Table | Holds | Lifetime |
| --- | --- | --- |
| `refresh_tokens` | session credentials | `JWT_REFRESH_TTL` (30 days) |
| `email_verification_tokens` | address-proof links | `AUTH_VERIFICATION_TTL` (24 h) |
| `password_reset_tokens` | reset links, single use | `AUTH_PASSWORD_RESET_TTL` (60 min) |

Rows survive being used or revoked — a revoked row is what makes a replayed
token detectable rather than merely unknown. Expired refresh tokens are swept
hourly by a background job started in `container.Build` and stopped on
shutdown, so the table stays bounded without a cron.

### Soft deletes

`models.Base` embeds `gorm.DeletedAt`, so `DELETE` marks rows instead of
removing them and every query filters them out automatically. Use
`db.Unscoped()` for a hard delete or to query deleted rows.

---

## Code generation

```bash
go run ./cmd/make scaffold Category      # or: make scaffold name=Category
```

Writes seven files and patches three registration points:

```
  created  internal/models/category.go
  created  internal/repositories/category_repository.go
  created  internal/requests/category_request.go
  created  internal/resources/category_resource.go
  created  internal/services/category_service.go
  created  internal/controllers/category_controller.go
  created  internal/services/category_service_test.go

  created  internal/database/migrations/20260813152336_create_categories_table.go
  wired    internal/container/container.go    +repo +service +field +wiring
  wired    internal/routes/api.go             +/api/v1/categories routes
```

Then edit the model's placeholder fields and run `make migrate`.

Single layers work too, and compose to the same result as `scaffold`:

```bash
go run ./cmd/make model Comment          # + its create-table migration
go run ./cmd/make service Comment
go run ./cmd/make controller Comment     # + container + routes
```

### Naming

Names are accepted in any spelling — `Category`, `category`, `blog_post`,
`BlogPost` — and normalised per target:

| Target | Convention | Example |
| --- | --- | --- |
| Go types | PascalCase | `BlogPost` |
| Filenames | snake_case | `blog_post_service.go` |
| Table names | snake_case plural | `blog_posts` |
| URLs | kebab-case plural | `/api/v1/blog-posts` |

Pluralization covers the regular rules (`Category` → `categories`, `Box` →
`boxes`) but not irregulars — `Person` becomes `persons`. Override with
`TableName()` on the model.

### How the wiring works

The generator inserts above `// codegen:` marker comments in `container.go`,
`api.go`, matching each marker's indentation. Migrations need no marker —
each file registers itself from an `init` function.

**Don't delete those markers.** Without one, the files still generate and the
tool prints the snippet for you to paste by hand.

Two safety properties: insertion checks whether the block is already present, so
re-running is a no-op rather than a duplicate; and every write goes through
`go/format`, so a patch that wouldn't parse is rejected instead of written.

### Doing it by hand

Nine steps, in order: model → migration → repository → service
→ request → resource → controller → container → routes.

---

## Renaming the project

Two separate "names" that change independently.

**Module path** — `github.com/scriptertoufiq/gobook`, what every import resolves
against. Roughly 60 occurrences across 20+ files:

```bash
go run ./cmd/make rename github.com/you/shop -dry-run   # preview
go run ./cmd/make rename github.com/you/shop
go mod tidy
```

**App name** — `APP_NAME`, only a label in the startup log and `/health`. It's
one line in `.env`; the flag exists to keep `.env`, `.env.example`, and the
fallback in `config.go` in agreement:

```bash
go run ./cmd/make rename -app-name "Shop API"
```

Both at once:

```bash
make rename module=github.com/you/shop app="Shop API"
```

Every `.go` file is re-parsed after substitution, so a rename that would break
the build is rejected before anything is written. It deliberately does **not**
rename the directory on disk or touch a git remote.

---

## Testing

```bash
make test          # go test ./... -race -count=1
```

The service layer is the part worth testing, and it runs without a database:
services depend on repository interfaces, so tests inject an in-memory fake.
`internal/services/user_service_test.go` covers password hashing, email
normalisation, duplicate rejection, partial updates, and the fact that
`Authenticate` returns an identical error for "wrong password" and "no such
user" so the endpoint can't be used to enumerate accounts.

`go run ./cmd/make scaffold X` generates a matching test file with its own fake
repository, so a new resource starts with coverage.

---

## Limitations and next steps

- **Access tokens survive a credential change** for up to `JWT_ACCESS_TTL` —
  including password changes, deactivation and deletion. Refresh tokens are
  revoked immediately, so the exposure is bounded at 15 minutes rather than 30
  days, but it is not zero. See [Authentication](#authentication).
- **Rate-limit state is per process.** Behind a load balancer each replica
  enforces its own share, so N replicas allow roughly N times the configured
  rate. Move the buckets to Redis when that stops being acceptable — the
  `ratelimit.Limiter` API is the seam.
- **CORS defaults to `*`.** Set `CORS_ALLOWED_ORIGINS` before deploying.
- **AutoMigrate is additive only** — migrations are versioned and ordered, but
  a migration whose `Up` is an `AutoMigrate` still can't drop or rename a
  column. Use `db.Migrator()` or raw SQL for those; see
  [Migrations](#migrations-and-seeding).
- **The scaffold's placeholder model** is `Name`/`Slug`/`Description`, and the
  generated service, repository, and tests all assume slug uniqueness. For
  something like `Comment` you'll delete the slug field and its `SlugTaken`
  method together.