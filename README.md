# SaúdeEmCasa (HealthAtHome)

**SaúdeEmCasa** is an On-Demand Home Healthcare Marketplace bridging the gap between families and licensed professionals in Portugal.
This repository is a **Monorepo** containing the Go Backend, React Frontend, and Infrastructure configurations.

## 🏗 Architecture
- **Frontend:** React (Vite) + TailwindCSS.
- **Backend:** Go (Golang) + Chi Router.
- **Database:** PostgreSQL.
- **Infrastructure:** Docker & Nginx.

## 📂 Project Structure
```text
/SAEMCA
├── /backend        # Go API & Worker Logic
├── /frontend       # React Application (Client/Pro/Admin Portals)
├── /infra          # Docker Compose & Nginx configs
└── /docs           # Technical Documentation & User Stories