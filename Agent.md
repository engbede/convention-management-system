# Agent.md — Convention Management System

You are assisting with development on the **Convention Management System**, a
web-based registration and administration platform built for the Methodist
Church Nigeria, Diocese of Apa, used for conventions, camps, and seminars.

Read this entire file before making any changes. If a request is ambiguous
or not covered here, ask before writing code — do not assume.

---

## TECH STACK (do not introduce a different one)

- **Language**: Go (Golang), standard library `net/http` — no web framework
  (no Gin/Echo/Fiber). Routing is done via `http.ServeMux` in `routes/routes.go`.
- **Templates**: Server-rendered HTML via Go's `html/template`, using a shared
  `admin_layout` template (`templates/layouts/`) with a sidebar + content block
  pattern (`{{template "content" .}}`).
- **CSS framework**: Bootstrap 5.3.3 + Bootstrap Icons 1.11.3, both loaded via
  CDN in the layout — not bundled, not npm-installed. Custom overrides live in
  `static/style.css`.
- **Database**: PostgreSQL in production (Neon), with SQLite support for
  local/dev (`database/sqlite.go`, `database/postgres.go`, dispatched via
  `database/init.go`). Migrations are hand-written Go functions
  (`database/migrate_postgres.go`, `database/migrate_sqlite.go`), not a
  migration library.
- **Auth**: Cookie-based session (`admin_session` cookie holding the admin's
  numeric ID), checked by `middleware.RequireAuth`. Passwords hashed with
  bcrypt. Single admin role only — no role-based access control yet.
- **Hosting**: Render, with a `/healthz` endpoint for health checks.
- **No JS framework** — this is not a React/Vue/SPA project. Interactivity is
  server-rendered pages and standard HTML forms/links. Do not introduce a
  frontend framework or client-side routing.

---

## ARCHITECTURE PATTERN (follow exactly, layer by layer)

Every feature follows this same layered structure — reuse it, don't deviate:

1. **`models/`** — plain Go structs describing the data shape (e.g.
   `Registration`, `Official`, `Notice`). No logic, just fields.
2. **`repository/`** — direct SQL queries against `database.DB`. Every table
   gets its own repository file(s) (e.g. `registration_repository.go`,
   `official_repository.go`). This is the only layer allowed to write SQL.
3. **`handlers/`** — HTTP handlers. Parse the request, call into
   `repository`/`services`, execute a template via the shared `Templates`
   variable, or redirect. No SQL in handlers.
4. **`validators/`** and **`validation/`** — plain functions returning `error`
   for field-level checks (e.g. `ValidatePhone`). Note: both folders exist;
   check which one a given feature already uses before adding to either —
   don't create a third pattern.
5. **`services/`** — cross-cutting concerns not tied to one table (QR code
   generation, SMS sending).
6. **`helpers/`** — small utility functions (e.g. registration number
   generation, QR code helpers).
7. **`templates/`** — one `.html` file per page, using `admin_layout` for
   anything behind auth, plus `templates/partials/` for shared fragments
   (header/footer).
8. **`routes/routes.go`** — every route is registered here, wrapped in
   `middleware.RequireAuth(...)` if it's an admin-only page. Public routes
   (registration form, login, health check) are not wrapped.

When adding a feature, touch all relevant layers in this order: model →
repository → handler → template → route. Don't skip the repository layer and
query the database directly from a handler.

---

## DESIGN SYSTEM

- Navbar: Bootstrap `bg-primary` (blue), brand text "Methodist Church
  Nigeria" + "Convention Management System" subtitle.
- Sidebar: dark (`bg-dark text-white`), Bootstrap List Group
  (`list-group-item list-group-item-action`) with Bootstrap Icons prefixing
  each label (e.g. `bi bi-speedometer2` for Dashboard).
- Main content area: `col-md-10 p-4` next to a `col-md-2` sidebar.
- Public registration form: a centered white card (`.form-container`,
  max-width 700px, rounded corners, soft shadow) over a full-bleed background
  image with a dark navy overlay (`rgba(8,35,85,.55)` over
  `church-bg.jpg`) — this look is specific to the public-facing form, don't
  apply it to admin pages.
- Reuse existing Bootstrap utility classes and the sidebar pattern for any
  new admin page — don't introduce a new visual style or a new CSS
  framework/component library.

---

## CURRENT STATE — WHAT'S BUILT

**Public:**
- `/` — Home/landing
- `/register` — registration form (`ShowForm`), tied to the currently active
  Convention (`repository.GetActiveConvention`)
- `/submit-registration` — form submission (`Register` handler), with
  validation via `validators/validation` packages
- `/success` — confirmation page

**Admin (all behind `RequireAuth`):**
- `/login`, `/logout` — bcrypt-checked session auth
- `/dashboard` — stats (total registrations, gender/membership/marital-status
  breakdowns, first-timers, check-in rate, per-circuit counts) pulled from
  `repository.GetDashboardStats` + `GetAttendanceStats` +
  `GetRegistrationsByCircuit`
- `/registrations`, `/view`, `/edit`, `/update`, `/delete` — full CRUD on
  registrations
- `/checkin` — marks a registration as checked-in (POST only)
- `/idcard`, `/print-idcards` — individual and bulk ID card generation, with
  QR codes (`helpers/qrcode.go`, `services/qrcode.go`)
- `/export/pdf`, `/export/excel` — registration exports
- `/conventions`, `/conventions/new|create|edit|update|activate|delete` —
  manage multiple Convention records, with one marked `Active` at a time
- `/officials` — full CRUD (`new|create|view|edit|update|delete`) for a
  separate `Official` entity (church officials/leaders, distinct from
  attendee Registrations)
- `/notices` — full CRUD for announcements (`Notice` model: title, message,
  audience, priority, pinned, date range)

## KNOWN GAP — WORTH FLAGGING, NOT ASSUMING FIXED

The README documents `/attendance` as a standalone "Check-in Dashboard"
route, but **no such route exists** in `routes/routes.go`, and there is no
`attendance.html` template. What actually happens: `repository.GetAttendanceStats`
computes checked-in/pending counts, and those numbers are folded directly into
the main `/dashboard` page — there is no separate attendance-focused view.
Check with the project owner about whether a real standalone `/attendance`
page (e.g. a check-in-focused list distinct from the general Registrations
list) is still wanted before building one — don't assume the README is
accurate over the actual code.

---

## SECURITY NOTES (already handled correctly here — keep it this way)

- `.gitignore` correctly excludes `.env`, `data/`, and `*.db` — unlike a
  companion project this same developer works on, there is no evidence of
  committed credentials here. Keep it that way: never commit `.env`, and
  never hardcode `DATABASE_URL`, `ADMIN_USERNAME`, or `ADMIN_PASSWORD`
  directly in code.
- Passwords are bcrypt-hashed (`models/admin.go` + seeding logic in
  `database/seed.go`) — never store or compare plaintext passwords.
- Only one admin role currently exists. The README lists "role-based access
  control" as a future improvement — don't assume it exists yet.

---

## WORKFLOW RULES (strict — follow exactly)

1. **One step at a time.** Give one file, or one small paired edit, per
   response. Do not bundle an entire feature into one giant reply.
2. After each step, **stop and wait** for confirmation that the change was
   made and tested before giving the next step.
3. **Never assume unstated requirements.** If a request is ambiguous, ask a
   clarifying question before writing code.
4. **Follow the existing layered architecture** (model → repository →
   handler → template → route) exactly as described above — don't introduce
   a different pattern, ORM, or framework partway through the project.
5. Reuse existing Bootstrap classes, the sidebar/layout pattern, and existing
   validator conventions. Don't invent new styling or a new validation
   pattern if an equivalent already exists.
6. When giving code, give the exact full block to paste — not a vague diff —
   unless the edit is a small, precise change to one existing file.
7. Don't touch unrelated modules (e.g. Officials, Notices, Conventions)
   while working on a different feature, unless the task explicitly requires
   it.

Wait for the project owner to describe which feature needs work before
proceeding.
