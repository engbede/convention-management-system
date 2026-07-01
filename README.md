# Convention Management System

A web-based Convention Management System built with Go (Golang) for managing attendee registrations, administration, attendance, reporting, and ID card generation for church conventions and similar events.

The system provides an easy-to-use registration form for attendees and a secure administrative dashboard for managing registrations, monitoring attendance, and generating reports.

---

# Features

## Public Features

- Online attendee registration
- Form validation
- Registration success confirmation
- Mobile-friendly interface

## Administrative Features

- Secure administrator login
- Dashboard with registration statistics
- Search registrations
- View attendee details
- Edit attendee information
- Delete registrations
- Check-in attendees
- Attendance statistics
- Registration by circuit
- QR Code generation
- Individual ID card generation
- Bulk ID card printing
- Export registrations to PDF
- Export registrations to Excel

## Security

- Password hashing using bcrypt
- Environment variable configuration
- PostgreSQL database support
- Health check endpoint for deployment monitoring

---

# Technology Stack

- Go (Golang)
- HTML Templates
- CSS
- PostgreSQL
- Neon Database
- Render Hosting
- Git & GitHub

---

# Project Structure

```
convention-management-system/
│
├── database/
├── handlers/
├── helpers/
├── middleware/
├── models/
├── repository/
├── routes/
├── static/
├── templates/
├── data/
├── main.go
├── go.mod
└── README.md
```

---

# Installation

## Clone the repository

```bash
git clone https://github.com/YOUR_USERNAME/convention-management-system.git

cd convention-management-system
```

---

## Install dependencies

```bash
go mod tidy
```

---

## Create a .env file

```env
DATABASE_URL=postgresql://username:password@host/database

PORT=8085

ADMIN_USERNAME=superadmin

ADMIN_PASSWORD=YourStrongPassword
```

---

## Run the application

```bash
go run .
```

The application will be available at:

```
http://localhost:8085
```

---

# Environment Variables

| Variable | Description |
|-----------|-------------|
| DATABASE_URL | PostgreSQL connection string |
| PORT | Server port |
| ADMIN_USERNAME | Default administrator username |
| ADMIN_PASSWORD | Default administrator password |

---

# Deployment

This project is designed for deployment on Render using a PostgreSQL database.

## Build Command

```bash
go build -o app .
```

## Start Command

```bash
./app
```

## Health Check

```
/healthz
```

## Required Environment Variables

```
DATABASE_URL
ADMIN_USERNAME
ADMIN_PASSWORD
PORT
```

---

# Default Routes

## Public Routes

| Route | Description |
|---------|-------------|
| / | Registration Form |
| /success | Registration Success Page |

## Authentication

| Route | Description |
|---------|-------------|
| /login | Administrator Login |
| /logout | Logout |

## Dashboard

| Route | Description |
|---------|-------------|
| /dashboard | Dashboard |
| /registrations | View Registrations |
| /view?id= | View Registration |
| /edit?id= | Edit Registration |
| /delete?id= | Delete Registration |
| /attendance | Check-in Dashboard |
| /checkin?id= | Mark Attendee Present |
| /export/pdf | Export PDF |
| /export/excel | Export Excel |
| /idcard?id= | Generate ID Card |
| /print-idcards | Print Multiple ID Cards |
| /healthz | Health Check |

---

# Database

The application automatically:

- Creates required tables
- Runs database migrations
- Seeds the initial administrator account

---

# Default Administrator

The administrator account is created automatically during the first startup using the following environment variables:

```
ADMIN_USERNAME

ADMIN_PASSWORD
```

If an administrator already exists, the seeding process is skipped.

---

# Future Improvements

- Multiple administrator accounts
- Role-based access control
- Password change page
- Username management
- Forgot password
- Email notifications
- SMS notifications
- QR code check-in
- Backup and restore
- Audit logs
- Multi-year convention management

---

# Version

Current Version:

```
v1.0.0
```

---

# License

This project is intended for use by the Methodist Church Nigeria, Diocese of Apa and may be adapted for church conferences, conventions, camps, seminars, and similar events.

---

# Author

Developed by:

** Ngbede Emmanuel **

Software Engineer

Powered by Go (Golang) and PostgreSQL.