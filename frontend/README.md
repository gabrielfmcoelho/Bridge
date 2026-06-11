# Bridge Frontend 🚀

A web console for **SSHCM** (SSH & Host Connection Manager), providing a unified management dashboard for infrastructure, hosts, DNS records, services, projects, secrets vault, topology mapping, and issue tracking.

Designed for high readability, modern micro-interactions, and visual consistency following the strict rules of the [Design System](src/DESIGN_SYSTEM.md).

---

## 🛠️ Tech Stack

This frontend is built using a modern, fast, and fully-typed stack:

- **Framework**: [Next.js 16 (App Router)](https://nextjs.org/) + [React 19](https://react.dev/) + [TypeScript](https://www.typescriptlang.org/)
- **Styling**: [Tailwind CSS v4](https://tailwindcss.com/) + Custom Design Tokens (CSS Variables)
- **Data Fetching & Caching**: [@tanstack/react-query (v5)](https://tanstack.com/query) for robust server-state synchronization
- **Form State & Validation**: [React Hook Form](https://react-hook-form.com/) + [Zod](https://zod.dev/) for typed schemas
- **Graph & Topology**: [@xyflow/react](https://reactflow.dev/) + [@dagrejs/dagre](https://github.com/dagrejs/dagre) for automated dependency layout calculations
- **Interactive Primitives**: [Vaul](https://github.com/emilkowalski/vaul) (bottom sheets/drawers) + [Radix UI](https://www.radix-ui.com/) (Popover, Tooltip)
- **Localization**: [Next-intl](https://next-intl-docs.vercel.app/) with full support for:
  - English (`en`)
  - Brazilian Portuguese (`pt-BR`)
- **API Documentation**: Integrated [@scalar/api-reference-react](https://github.com/scalar/scalar) for interactive API browsing

---

## ✨ Key Features

- **📊 Unified Dashboard**: Aggregated stats for hosts, DNS records, projects, and services. Displays scan coverage, alerts, hosting provider distribution, and recent scans.
- **🖥️ Host Inventory & Scans**: Complete remote system dashboard displaying CPU, RAM, Disk usage, OS details, last logins, containers, active ports, systemd services, packages, and cron jobs.
- **🔑 SSH Keys Management**: Direct SSH key generation and provisioning, public key fingerprinting, and deployment tracking.
- **🔒 Secure Secrets Vault**: Personal vs. Shared credentials vault with advanced client-side filtering (scope, visibility, type). Custom auto-hide reveal UX, clipboard auto-clear, and derived single-use share links (HKDF AES-256).
- **🕸️ Service Topology Mapping**: Visually inspect dependencies between services, hosts, and projects through custom interactive nodes and edges.
- **📋 Issues & Releases Board**: Kanban board for problem tracking (`Acontecimentos`) and release cycle management.
- **📞 Centralized Contacts (Responsáveis)**: Assign multiple internal/external team members with role titles, phone masks, and quick WhatsApp triggers.

---

## 📁 Directory Structure

```text
src/
├── app/                  # Next.js App Router pages (hosts, secrets, topology, etc.)
│   ├── layout.tsx        # Base root layout & provider injectors
│   ├── page.tsx          # Main dashboard landing page
│   └── [feature]/        # Feature subfolders with pages and component modules
├── components/           # UI and inventory-specific components
│   ├── inventory/        # Reusable card header, metadata grid, and tags layout
│   └── ui/               # Primary components (Card, Button, Badge, Drawer, etc.)
├── contexts/             # Global contexts (Locale, Auth, Theme)
├── hooks/                # Custom React hooks (caching, timers, state utilities)
├── lib/                  # Utilities, API client definitions, and constant tokens
│   ├── api.ts            # Typed REST client matching backend structures
│   ├── constants.ts      # Entity colors mapping, status fallback tokens
│   └── types.ts          # Central TypeScript interfaces matching Go structs
└── messages/             # Translation strings (en.json, pt-BR.json)
```

---

## 🎨 Design System

All developers and agents MUST follow the conventions documented in [DESIGN_SYSTEM.md](src/DESIGN_SYSTEM.md).

Key guidelines include:
1. **Card Anatomy**: Every card in lists must have header, metadata grid, tags, domain-specific content, and bottom indicators (exactly 5 vertical blocks).
2. **Entity Colors**: Respect the strict color assignments (e.g. Cyan for Hosts, Emerald for DNS, Amber for Services, Violet for Projects).
3. **Typography**: Use `--font-display` (JetBrains Mono) for page titles and `--font-mono` for IDs, domains, hostnames, and IP addresses.
4. **Responsaveis Pattern**: Always handle multiple contacts using `<ResponsavelList>` and `<ResponsaveisSection>`.

---

## 🚀 Getting Started

### Prerequisites

- **Node.js**: `v22` (recommended)
- **Backend API**: Make sure the backend server (Go) is running, typically on `http://localhost:8080`.

### Environment Configuration

The dev server uses Next.js rewrites to proxy API requests under `/api/` to the backend. You can customize the target backend address using the following environment variables:

Create a `.env` (or `.env.local`) file:

```env
API_URL=http://localhost:8080
NEXT_PUBLIC_API_URL=
```

### Installation

Install packages using `npm`:

```bash
npm install
```

### Running the Development Server

Start the local server with hot-reloading:

```bash
npm run dev
```

Open [http://localhost:3000](http://localhost:3000) in your browser.

### Linting & Building

Ensure TypeScript types and lint checks pass, then build for production:

```bash
# Run ESLint rules
npm run lint

# Build standalone distribution assets
npm run build

# Start the compiled production build
npm run start
```

---

## 🐳 Docker Deployment

The application compiles into an optimized, lightweight `standalone` production bundle.

To run via Docker:

```bash
# Build the Docker image (from the root of the workspace)
docker build -f Dockerfile.frontend -t bridge-frontend .

# Run the container
docker run -p 3000:3000 -e API_URL=http://your-backend-api:8080 bridge-frontend
```
