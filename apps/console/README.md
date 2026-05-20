# GoGoMail Admin Console

Enterprise administration console for GoGoMail platform built with Next.js 15 and Cloudscape Design System.

## Quick Start

### Prerequisites

- Node.js 20+
- pnpm 8+

### Installation

```bash
cd apps/console
pnpm install
```

### Development

```bash
# Start dev server (port 3001)
pnpm dev

# Backend must be running on http://localhost:8080
# or set GOGOMAIL_BACKEND_URL environment variable
```

Navigate to http://localhost:3001 and log in with your admin credentials.

### Build

```bash
pnpm build
pnpm start
```

### Type Checking

```bash
pnpm type-check
```

### Testing

```bash
# Unit tests
pnpm test

# E2E tests
pnpm test:e2e
```

## Architecture

### BFF (Backend for Frontend) Pattern

The admin console uses a stateless BFF pattern for horizontal scaling:

- **API Routes**: `/app/api/` — Next.js API Routes act as stateless proxy to Go backend
- **Auth**: httpOnly JWT cookies (XSS protection, stateless)
- **Token Refresh**: Silent refresh on 401 response
- **No Sessions**: Load balancer can route to any BFF instance

### Directory Structure

```
src/
├── app/
│   ├── (auth)/login/page.tsx      # Login page
│   ├── (console)/                 # Protected routes
│   │   ├── layout.tsx             # Cloudscape AppLayout
│   │   ├── dashboard/
│   │   ├── users/
│   │   ├── domains/
│   │   ├── audit-logs/
│   │   └── ...
│   ├── api/
│   │   ├── auth/                  # Login/Logout/Refresh
│   │   └── admin/[...path]/       # Generic proxy to backend
│   ├── layout.tsx                 # Root layout + Providers
│   └── middleware.ts              # Auth guard
│
├── components/                    # Reusable UI components
├── hooks/                         # React Query hooks
├── lib/                           # Utilities (api-client, etc.)
└── types/                         # TypeScript definitions
```

### Key Technologies

- **UI Framework**: Cloudscape Design System v3 (AWS-style)
- **Server State**: React Query v5
- **Styling**: Tailwind CSS v4
- **Authentication**: JWT (httpOnly cookies)
- **Type Safety**: TypeScript + OpenAPI-generated types

## Backend Integration

All admin console API requests go through the BFF proxy at `/api/admin/[...path]`:

```
Browser → BFF (/api/admin/*) → Go Backend (/admin/v1/*)
         (httpOnly JWT)      (Bearer token)
```

The BFF handles:
- Cookie-to-Bearer token conversion
- Silent token refresh (401 responses)
- Error normalization
- CORS headers

Current launch-readiness UI includes cursor-paginated audit logs, filterable
delivery attempts with visible success/error feedback, and typed proxy/error
helpers that are covered by `pnpm type-check`.

## Environment Variables

```bash
GOGOMAIL_BACKEND_URL=http://localhost:8080  # Backend URL (default)
NODE_ENV=development|production             # Environment
GIT_SHA=abc123def                           # Git commit SHA (for build ID)
```

## Performance Optimization

- Cloudscape components are optimized via `experimental.optimizePackageImports`
- Heavy charts/components use dynamic imports with `ssr: false`
- React Query caching: staleTime varies by data freshness needs
- No in-memory sessions or sticky sessions required

## Development Guidelines

1. **Types**: Always use `@gogomail/api-types` for backend schemas
2. **API**: Use `apiClient` utilities for all backend calls
3. **State**: Prefer React Query over local state for server data
4. **Testing**: Component tests with Vitest, E2E with Playwright
5. **Styling**: Tailwind first, Cloudscape components for complex layouts

## Future Enhancements

- [ ] WebSocket support for real-time updates
- [ ] Chart visualization for statistics
- [ ] Bulk operations for users/domains
- [ ] API metering dashboard
