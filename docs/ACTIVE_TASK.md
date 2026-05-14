# ACTIVE_TASK

## PARALLEL WORK: SMTP Optimization (Super-Powerful SMTP)

### Background

TASK-067 (Audit Logs) completion enables SMTP monitoring. However, SMTP itself lacks:
- Monster performance & scalability for massive traffic
- Extreme stability (bulk mail doesn't impact regular users)
- Mode-based multiplexing (single instance, multiple roles)
- Server farm configuration (horizontal scaling)

### Completed Phases

**Phase 1 ✓ — Connection & Concurrency Control**
- [x] Increased SMTPMaxConnections default from 0 (unlimited) to 10,000
- Load testing validates <100ms latency and >95% success rate under 1000 concurrent connections

**Phase 2 ✓ — Bulk Mail Isolation**
- [x] BulkSenderLimiter with token bucket rate limiting (100 msg/sec default)
- [x] Regular users unaffected while bulk senders isolated
- [x] LoadTest validates bulk traffic doesn't impact regular user latency

**Phase 3 ✓ — Memory Optimization**
- [x] HeaderBuffer collects all headers (Received, Message-ID, Authentication-Results) in memory
- [x] Single-pass file rewrite instead of three separate rewrites per message
- [x] Eliminates I/O pressure and GC strain for large messages (25MB+)
- [x] Single temp file created instead of 3, reducing disk thrashing

### Current Phase: Phase 4 — Delivery Concurrency Control

Next: Per-domain delivery concurrency limits and circuit breaker pattern

---

## TASK-068: Identity Provider Abstraction

### 배경

User and group provisioning is currently database-only (hardcoded in users/groups tables).
The backend-roadmap requires a pluggable identity provider abstraction to support:
1. Database-only (default, current state)
2. LDAP/Active Directory (via bind, search, sync)
3. Azure AD (via Microsoft Graph API)
4. External RDBMS (SQL query against remote database)

Per-domain IdP configuration allows each domain to use a different identity source,
enabling organizations to federate multiple identity providers in a single deployment.

TASK-068 defines the `IdentityProvider` interface and pluggable provider registry,
allowing downstream TASK-069/070/071/072/073 to implement concrete providers.

### 구현 대상

- `internal/idprovider/interface.go` — `IdentityProvider` interface (User/Group CRUD, search, sync)
- `internal/idprovider/registry.go` — pluggable provider registry (Get by type, Register)
- `internal/idprovider/models.go` — User, Group, Member domain models (aligned with maildb)
- `internal/idprovider/database/provider.go` — Database provider implementation (wraps maildb)
- Database schema update for `idp_configurations` table (per-domain IdP config)
- `internal/idprovider/*_test.go` — unit tests for interface, registry, database provider
- Update `maildb.User` and `maildb.Group` exports for idprovider import
- `docs/ACTIVE_TASK.md`
- `docs/CURRENT_STATUS.md`
- `docs/NEXT_STEPS.md`

### 완료 조건

- [ ] `IdentityProvider` interface defined with methods: GetUser, GetGroup, ListUsers, ListGroups, CreateUser, UpdateUser, DeleteUser, CreateGroup, DeleteGroup, AddMember, RemoveMember
- [ ] `IdentityProviderRegistry` with Get(providerType string) and Register(providerType string, provider IdentityProvider)
- [ ] `idprovider.User` and `idprovider.Group` domain models align with maildb.User/Group structure
- [ ] Database provider implements IdentityProvider by wrapping maildb repositories
- [ ] `idp_configurations` table stores per-domain IdP configuration (domain_id, provider_type, config jsonb)
- [ ] Database migration creates idp_configurations table with domain_id foreign key
- [ ] Admin can query current IdP configuration for a domain via repository method
- [ ] Unit tests for IdentityProvider interface contract (CRUD, search)
- [ ] Unit tests for registry (register, get, unknown provider error)
- [ ] Unit tests for database provider (wraps maildb, returns correct user/group)
- [ ] `go test -count=1 ./internal/idprovider ./internal/idprovider/database -v` 통과
- [ ] `go test ./...` 통과
- [ ] 개발 문서를 최신 상태로 갱신한다.

### 다음 태스크

TASK-069: Database Identity Mode
