# gogomail 배포 가이드

> **대상** — gogomail용 docker-compose(또는 k8s) 매니페스트를 생성하는
> 운영 엔지니어와 AI 에이전트. 본 문서는 env 변수, 포트, 사이징, DNS
> 레코드에 대한 단일 진실의 출처입니다. 모드별 세부 사항은
> [`docs/MODES.md`](../docs/MODES.md), 위협 모델은
> [`docs/SECURITY.md`](../docs/SECURITY.md)를 참조하세요.
>
> English / 영어: [DEPLOYMENT.md](DEPLOYMENT.md)

---

## 목차

1. [소개](#1-소개)
2. [필수 인프라](#2-필수-인프라)
3. [사이징 의사결정 트리](#3-사이징-의사결정-트리)
4. [모드-컨테이너 매핑](#4-모드-컨테이너-매핑)
5. [필수 env 변수](#5-필수-env-변수)
6. [Compose 레시피](#6-compose-레시피)
7. [네트워크 노출 매트릭스](#7-네트워크-노출-매트릭스)
8. [DNS 설정](#8-dns-설정)
9. [TLS / 인증서](#9-tls--인증서)
10. [초기 설정](#10-초기-설정)
11. [운영](#11-운영)
12. [AI 에이전트용 지침](#12-ai-에이전트용-지침)

---

## 1. 소개

gogomail은 시작 시 역할을 선택하는 단일 정적 Go 바이너리로 제공됩니다.
모드는 총 **24개**입니다 ([`docs/MODES.md`](../docs/MODES.md)). 본
가이드는 Docker Compose로 운영 배포를 구성하는 방법을 설명합니다.
동일한 env 변수와 포트가 Kubernetes Deployment에도 그대로 매핑됩니다.

범위:
- 하드 인프라 요구사항과 최소 버전
- 토폴로지 선택을 위한 의사결정 트리
- 모드-컨테이너 참조 표
- env 변수 전체 참조
- 네 가지 즉시 사용 가능한 compose 레시피 (single, small, medium, large)
- 필수 DNS, TLS, 초기 부트스트랩 절차
- 마지막에 AI 에이전트용 명시적 지침

---

## 2. 필수 인프라

| 컴포넌트 | 최소 버전 | 비고 |
|---|---|---|
| **PostgreSQL** | 16+ | 확장: `pg_trgm`, `uuid-ossp`, `pgcrypto`. HA: streaming replication 또는 Patroni. |
| **Redis** | 7+ | 단일/Sentinel/Cluster. KeyDB·Dragonfly(Redis 7 프로토콜) 가능. 프로덕션에서는 `requirepass` 필수. |
| **S3 호환 스토리지** | MinIO 8+, AWS S3, Backblaze B2, Cloudflare R2, Wasabi | 프로덕션에서 HTTPS 엔드포인트 필수 (검증기가 HTTP 거부). SSE 권장. |
| **리버스 프록시 / LB** | nginx 1.24+, Caddy 2.7+, HAProxy 2.8+, Traefik 3+ | TLS 종료, HSTS, `X-Forwarded-*`. SMTP L4 분산에는 HAProxy 권장. |
| **런타임** | Docker 24+ 또는 k8s 1.28+ | 컨테이너는 distroless non-root. 1024 미만 포트에는 `CAP_NET_BIND` 또는 호스트 매핑 필요. |
| **OpenSearch** (선택) | OpenSearch 2.11+ / Elasticsearch 8+ | `GOGOMAIL_SEARCH_INDEX_BACKEND=opensearch` 일 때만. |
| **ClamAV** (선택) | clamd 1.0+ | `GOGOMAIL_ATTACHMENT_SCAN_BACKEND=clamav` 일 때만. |

**서버 리소스 (역할별 권장 최소치)**:

| 역할 | CPU | RAM | 디스크 |
|---|---|---|---|
| all-in-one | 2 vCPU | 2 GiB | 10 GiB + mailstore |
| edge / inbound / outbound MTA | 0.5 vCPU | 512 MiB | 1 GiB |
| imap | 1 vCPU | 1 GiB | 1 GiB |
| mail-api / admin-api / auth-server | 1 vCPU | 1 GiB | 1 GiB |
| delivery-worker | 0.5 vCPU | 512 MiB | 1 GiB |
| outbox-relay (singleton) | 0.5 vCPU | 256 MiB | 1 GiB |
| Postgres (medium) | 4 vCPU | 16 GiB | 100+ GiB SSD |
| Redis (medium) | 2 vCPU | 4 GiB | AOF 영구화 |
| MinIO 노드 (medium) | 2 vCPU | 4 GiB | 1+ TiB |

---

## 3. 사이징 의사결정 트리

```
Q1: 메일박스 < 50, 내부 전용, 단일 호스트로 충분?
    yes -> Pattern A: 단일 노드 (docker-compose.small.yml 기반)
    no  -> Q2

Q2: 메일박스 < 5k, 일일 메일 < 50k, 단일 AZ 허용?
    yes -> Pattern B: 소형 (앱 호스트 2 + 워커 호스트 1 + 관리형 PG/Redis/S3)
    no  -> Q3

Q3: 메일박스 < 50k, 일일 메일 < 500k, 단일 리전 multi-AZ?
    yes -> Pattern C: 중형 (역할 분리: edge / app / worker / index)
    no  -> Pattern D: 대형 (전체 모드 분리, multi-DC, k8s)
```

| 패턴 | 메일박스 | 일일 메일 | 호스트 | Compose 파일 |
|---|---:|---:|---:|---|
| A single-node | < 500 | < 5k | 1 | `docker-compose.small.yml` |
| B small | < 5k | < 50k | 3-5 | `small.yml` 변형 |
| C medium | < 50k | < 500k | 15-25 | `docker-compose.medium.yml` |
| D large | 50k+ | 500k+ | k8s | `docker-compose.large.yml` |

확실하지 않으면 한 단계 작게 시작하고 스케일 아웃하세요. 코드/스키마
변경 없이 컨테이너 추가와 env 변경만으로 상위 패턴으로 전환 가능합니다.

---

## 4. 모드-컨테이너 매핑

24개 모드 전체. `APP_MODE`가 정식 선택자입니다 (`-mode` CLI 플래그와 동등).

| 모드 | `APP_MODE` | 복제본 (최소/권장) | 메모리 | CPU | 노출 포트 | 필수 의존성 | 스케일 규칙 | 헬스 |
|---|---|---|---|---|---|---|---|---|
| All-in-one | `all-in-one` | 1 / 2 | 1 GiB | 1 | 8080 | PG, Redis, S3 | stateless | `GET /health/ready` |
| Edge MTA | `edge-mta` | 2 / 3+ | 512 MiB | 0.5 | 25 (또는 2525) | PG, Redis | stateless | TCP `:25` |
| Inbound MTA (trusted) | `inbound-mta` | 2 / 2 | 256 MiB | 0.25 | 2526 (내부) | PG, Redis | stateless | TCP |
| Outbound MTA (submission) | `outbound-mta` | 2 / 3 | 512 MiB | 0.5 | 587 + 465 | PG, Redis | stateless | TCP |
| Delivery worker | `delivery-worker` | 2 / 3+ | 512 MiB | 0.5 | — | PG, Redis | consumer-group 샤딩 | 메트릭 |
| IMAP | `imap` | 2 / 3+ | 1 GiB | 1 | 143 + 993 | PG, Redis, S3 | stateless, sticky-by-IP 선택 | TCP |
| POP3 | `pop3` | 2 / 2 | 512 MiB | 0.5 | 110 + 995 | PG, S3 | stateless | TCP |
| CalDAV | `caldav` | 2 / 2 | 512 MiB | 0.5 | 8081 | PG | stateless | TCP |
| CardDAV | `carddav` | 2 / 2 | 512 MiB | 0.5 | 8082 | PG | stateless | TCP |
| WebDAV (Drive) | `webdav` | 2 / 2 | 512 MiB | 0.5 | 8083 | PG, S3 | stateless | TCP |
| LDAP gateway | `ldap-gateway` | 2 / 2 | 512 MiB | 0.5 | 389 + 636 | PG | stateless | TCP |
| Mail API | `mail-api` | 2 / 3+ | 1 GiB | 1 | 8080 | PG, Redis, S3 | stateless | `GET /health/ready` |
| Admin API | `admin-api` | 2 / 2 | 1 GiB | 1 | 8080 | PG, Redis | stateless | `GET /health/ready` |
| Auth server | `auth-server` | 2 / 2 | 512 MiB | 0.5 | 8080 | PG, Redis | stateless | `GET /health/ready` |
| Outbox relay | `outbox-relay` | 2 / 2 | 256 MiB | 0.25 | — | PG, Redis | **싱글톤 (PG advisory lock)** — 복제본은 failover 전용 | `outbox_lag` 메트릭 |
| Event worker | `event-worker` | 2 / 2 | 512 MiB | 0.5 | — | PG, Redis | consumer-group | 메트릭 |
| Search index worker | `search-index-worker` | 2 / 2 | 1 GiB | 1 | — | PG, Redis, OpenSearch | consumer-group | 메트릭 |
| Push notification worker | `push-notification-worker` | 2 / 2 | 512 MiB | 0.5 | — | PG, Redis | consumer-group | 메트릭 |
| API metering worker | `api-metering-worker` | 2 / 2 | 512 MiB | 0.5 | — | PG, Redis | consumer-group | 메트릭 |
| Attachment cleanup | `attachment-cleanup-worker` | 1 / 2 | 256 MiB | 0.25 | — | PG, S3 | **싱글톤** | 메트릭 |
| Drive cleanup | `drive-cleanup-worker` | 1 / 2 | 256 MiB | 0.25 | — | PG, S3 | **싱글톤** | 메트릭 |
| DAV sync retention | `dav-sync-retention-worker` | 1 / 1 | 256 MiB | 0.25 | — | PG | **싱글톤**, dry-run 기본 | 메트릭 |
| API usage retention | `api-usage-retention-worker` | 1 / 1 | 256 MiB | 0.25 | — | PG | **싱글톤**, dry-run 기본 | 메트릭 |
| Batch worker | `batch-worker` | 1 / 2 | 256 MiB | 0.25 | — | PG | **잡별 싱글톤** | 메트릭 |

**참고**:
- 싱글톤은 Postgres advisory lock (또는 Redis lease) 으로 선출. failover
  목적으로 2 복제본 운영 권장. 활성은 한 번에 하나.
- 각 모드는 필요한 env 변수만 읽으므로 모든 컨테이너에 슈퍼셋을 전달해도
  무방.
- SMTP/IMAP/POP3 평문 기본 포트는 `2525 / 1143 / 1110` (non-root 바인딩).
  호스트 포트 매핑으로 25 / 143 / 110 노출.
- `outbox-relay`는 글로벌. multi-DC에서는 PG primary가 있는 DC에서만 활성.

### 의존 그래프 (기동 순서)

```
postgres ──┐
redis ─────┼──> outbox-relay ──> delivery/search/push/api-metering 워커
s3/minio ──┘    │
                ├──> edge-mta, outbound-mta, mail-api, admin-api, auth-server
                ├──> imap, pop3, caldav, carddav, webdav, ldap-gateway
                └──> cleanup / retention / batch (싱글톤)
```

---

## 5. 필수 env 변수

검증기가 읽는 모든 env 변수. **R** = 해당 범위에서 필수, **O** = 선택
(기본값 표기). "prod" = `GOGOMAIL_ENV=production`.

### 5.1 Core

| Env | 범위 | 기본 | 형식 | 예시 | 비고 |
|---|---|---|---|---|---|
| `GOGOMAIL_ENV` | R | `development` | enum `development/test/production` | `production` | 검증기 활성화. |
| `APP_MODE` | R | `all-in-one` | enum (§4) | `mail-api` | `-mode` 플래그와 동등. |
| `GOGOMAIL_PUBLIC_BASE_URL` | R prod | _빈 값_ | 절대 URL | `https://mail.example.com` | prod에서 https 필수, localhost 금지. |
| `GOGOMAIL_HTTP_ADDR` | O | `:8080` | host:port | `:8080` | API/admin/auth 리스너. |
| `GOGOMAIL_CORS_ALLOWED_ORIGINS` | O | _빈 값_ | CSV | `https://webmail.example.com` | 브라우저 앱용. |

### 5.2 Database

| Env | 범위 | 기본 | 형식 | 예시 | 비고 |
|---|---|---|---|---|---|
| `GOGOMAIL_DATABASE_URL` | R | `postgres://gogomail:gogomail@localhost:5432/gogomail?sslmode=disable` | DSN | `postgres://gogomail:secret@pg:5432/gogomail?sslmode=require` | prod에서 `sslmode=disable` 금지. |
| `GOGOMAIL_DB_MAX_OPEN_CONNS` | O | `20` | int | `40` | 복제본당. |
| `GOGOMAIL_DB_MAX_IDLE_CONNS` | O | `5` | int | `10` | |
| `GOGOMAIL_DB_CONN_MAX_LIFETIME` | O | `30m` | duration | `30m` | |
| `GOGOMAIL_DB_CONN_MAX_IDLE_TIME` | O | `5m` | duration | `5m` | |
| `GOGOMAIL_MIGRATION_DIR` | O | `/app/migrations` | path | _이미지 기본_ | Dockerfile에서 설정. |

### 5.3 Redis

| Env | 범위 | 기본 | 형식 | 예시 | 비고 |
|---|---|---|---|---|---|
| `GOGOMAIL_REDIS_ADDR` | R (단일) | `localhost:6379` | host:port | `redis:6379` | 또는 Sentinel. |
| `GOGOMAIL_REDIS_PASSWORD` | R prod | _빈 값_ | string | _32+ 랜덤_ | farm coordinator=redis 일 때 필수. |
| `GOGOMAIL_REDIS_SENTINEL_ADDRS` | O | _빈 값_ | CSV host:port | `s1:26379,s2:26379,s3:26379` | Sentinel HA. |
| `GOGOMAIL_REDIS_MASTER_NAME` | O | `mymaster` | string | `mymaster` | Sentinel master 이름. |
| `GOGOMAIL_FARM_COORDINATOR_BACKEND` | R prod | `noop` | enum `noop/redis` | `redis` | **prod에서 `redis` 필수.** |
| `GOGOMAIL_FARM_COORDINATOR_HEARTBEAT_TTL` | O | `30s` | duration | `30s` | 싱글톤 lease TTL. |
| `GOGOMAIL_FARM_COORDINATOR_JOB_VISIBILITY_TIMEOUT` | O | `5m` | duration | `5m` | |

### 5.4 Storage (S3 / MinIO / local)

| Env | 범위 | 기본 | 형식 | 예시 | 비고 |
|---|---|---|---|---|---|
| `GOGOMAIL_STORAGE_BACKEND` | R | `local` | enum `local/nfs/s3/minio` | `s3` | |
| `GOGOMAIL_STORAGE_S3_ENDPOINT` | R (s3/minio) | _빈 값_ | URL | `https://minio:9000` | **prod에서 HTTPS 필수.** |
| `GOGOMAIL_STORAGE_S3_REGION` | R (s3/minio) | `us-east-1` | string | `us-east-1` | |
| `GOGOMAIL_STORAGE_S3_BUCKET` | R (s3/minio) | _빈 값_ | bucket | `gogomail` | 존재해야 함; §10. |
| `GOGOMAIL_STORAGE_S3_PREFIX` | O | _빈 값_ | path | `prod/` | |
| `GOGOMAIL_STORAGE_S3_ACCESS_KEY_ID` | R (s3/minio) | _빈 값_ | string | `AKIA…` | |
| `GOGOMAIL_STORAGE_S3_SECRET_ACCESS_KEY` | R (s3/minio) | _빈 값_ | string | _secret_ | |
| `GOGOMAIL_STORAGE_S3_FORCE_PATH_STYLE` | O | `false` | bool | `true` | MinIO에서 `true`. |
| `GOGOMAIL_STORAGE_S3_CA_CERT_FILE` | O | _빈 값_ | path | `/certs/ca.pem` | 자체서명 CA. |
| `GOGOMAIL_STORAGE_S3_INSECURE_SKIP_VERIFY` | _prod 금지_ | `false` | bool | `false` | prod에서 false 필수. |
| `GOGOMAIL_MAILSTORE_ROOT` | R (local/nfs) | `var/mailstore` | path | `/var/lib/gogomail/mailstore` | |

### 5.5 Auth + admin

| Env | 범위 | 기본 | 형식 | 예시 | 비고 |
|---|---|---|---|---|---|
| `GOGOMAIL_AUTH_JWT_SECRET` | R prod | _빈 값_ | string ≥32 byte | `openssl rand -base64 32` | **HS256 시크릿; prod ≥32 바이트.** |
| `GOGOMAIL_ADMIN_TOKEN` | R prod | _빈 값_ | string ≥32 byte | _랜덤_ | 관리자 자동화용 bearer. |
| `GOGOMAIL_ADMIN_MFA_REQUIRED` | O | `false` | bool | `true` | 관리자 TOTP 강제. |
| `GOGOMAIL_SCIM_TOKEN` | O | _빈 값_ | string | _bearer_ | SCIM 2.0 활성화. |
| `GOGOMAIL_SCIM_DEFAULT_DOMAIN_ID` | SCIM 시 O | _빈 값_ | UUID | `…` | |

### 5.6 SMTP / submission / delivery

| Env | 범위 | 기본 | 형식 | 예시 | 비고 |
|---|---|---|---|---|---|
| `GOGOMAIL_SMTP_ADDR` | R (edge) | `:2525` | host:port | `:2525` | 호스트에서 25→2525 매핑. |
| `GOGOMAIL_SMTP_DOMAIN` | R | `localhost` | hostname | `mail.example.com` | **prod에서 localhost 금지.** |
| `GOGOMAIL_SMTP_TLS_CERT_FILE` | R prod TLS | _빈 값_ | path | `/certs/smtp.pem` | STARTTLS. |
| `GOGOMAIL_SMTP_TLS_KEY_FILE` | R prod TLS | _빈 값_ | path | `/certs/smtp.key` | |
| `GOGOMAIL_SMTP_DMARC_ENFORCEMENT` | O | `reject` | enum `monitor/quarantine/reject` | `reject` | |
| `GOGOMAIL_SMTP_AUTH_VERIFICATION_ENABLED` | O | `false` | bool | `true` | SPF/DKIM/DMARC 검증. |
| `GOGOMAIL_SMTP_MAX_CONNECTIONS` | O | `10000` | int | `10000` | |
| `GOGOMAIL_SMTP_MAX_MESSAGE_BYTES` | O | `26214400` (25 MiB) | int64 | `52428800` | |
| `GOGOMAIL_SUBMISSION_ADDR` | R (outbound) | `:2587` | host:port | `:2587` | 587 매핑. |
| `GOGOMAIL_SUBMISSION_SMTPS_ADDR` | O | _빈 값_ | host:port | `:2465` | implicit-TLS 465; TLS 파일 필요. |
| `GOGOMAIL_SUBMISSION_ALLOW_INSECURE_AUTH` | _prod 금지_ | `false` | bool | `false` | |
| `GOGOMAIL_DELIVERY_SMTP_HELLO` | R | _빈 값_ | hostname | `mail.example.com` | 공개 hostname; prod에서 localhost 금지. |
| `GOGOMAIL_DELIVERY_TLS_MODE` | O | `opportunistic` | enum `opportunistic/require/disable` | `opportunistic` | |
| `GOGOMAIL_DELIVERY_SMARTHOST` | O | _빈 값_ | host:port | `smtp.sendgrid.net:587` | 스마트호스트 릴레이. |
| `GOGOMAIL_DELIVERY_RETRY_DELAYS` | O | _기본 내장_ | CSV duration | `1m,5m,15m,1h,6h,24h` | |
| `GOGOMAIL_DKIM_ENABLED` | O | `false` | bool | `true` | 발신 DKIM 서명. |
| `GOGOMAIL_DNSBL_ZONES` | O | _빈 값_ | CSV | `zen.spamhaus.org` | |
| `GOGOMAIL_INBOUND_TRUSTED_RELAYS` | O | `127.0.0.1/32,::1/128` | CSV CIDR | `10.0.0.0/8` | `inbound-mta` 전용. |
| `GOGOMAIL_RCPT_RATE_LIMIT_PER_MINUTE` | O | `60` | int >0 | `60` | |
| `GOGOMAIL_RATELIMIT_BACKEND` | O | `none` | enum `none/redis` | `redis` | prod에서 redis. |
| `GOGOMAIL_BACKPRESSURE_BACKEND` | O | `none` | enum `none/redis` | `redis` | |

### 5.7 IMAP / POP3 / DAV / LDAP

| Env | 범위 | 기본 | 형식 | 예시 | 비고 |
|---|---|---|---|---|---|
| `GOGOMAIL_IMAP_ADDR` | R (imap) | `:1143` | host:port | `:1143` | 143→1143. |
| `GOGOMAIL_IMAP_TLS_CERT_FILE` / `_KEY_FILE` | R (STARTTLS) | _빈 값_ | path | `/certs/imap.pem` | |
| `GOGOMAIL_IMAP_ALLOW_INSECURE_AUTH` | _prod 금지_ | `false` | bool | `false` | |
| `GOGOMAIL_IMAP_MAX_CONNECTIONS` | O | `5000` | int | `5000` | |
| `GOGOMAIL_IMAP_IDLE_TIMEOUT` | O | `30m` | duration | `30m` | |
| `GOGOMAIL_POP3_ADDR` | R (pop3) | `:1110` | host:port | `:1110` | |
| `GOGOMAIL_POP3S_ADDR` | O | _빈 값_ | host:port | `:1995` | implicit TLS. |
| `GOGOMAIL_POP3_MAX_CONNECTIONS` | O | `2000` | int | `2000` | |
| `GOGOMAIL_CALDAV_ADDR` | R (caldav) | `:8081` | host:port | `:8081` | |
| `GOGOMAIL_CALDAV_ALLOW_INSECURE_AUTH` | _prod 금지_ | `false` | bool | `false` | |
| `GOGOMAIL_CALDAV_TRUSTED_PROXIES` | O | _빈 값_ | CSV CIDR | `10.0.0.0/8` | `X-Forwarded-For` 신뢰. |
| `GOGOMAIL_CALDAV_TRUST_FORWARDED_PROTO` | O | `false` | bool | `true` | TLS 종료 LB 뒤. |
| `GOGOMAIL_CALDAV_SCHEDULING` | O | `false` | bool | `true` | RFC 6638. |
| `GOGOMAIL_CARDDAV_ADDR` | R (carddav) | `:8082` | host:port | `:8082` | |
| `GOGOMAIL_CARDDAV_ALLOW_INSECURE_AUTH` | _prod 금지_ | `false` | bool | `false` | |
| `GOGOMAIL_WEBDAV_ADDR` | R (webdav) | `:8083` | host:port | `:8083` | |
| `GOGOMAIL_WEBDAV_DEPTH_INFINITY_ENABLED` | O | `false` | bool | `false` | DoS 보호; off 권장. |
| `GOGOMAIL_LDAP_ADDR` | R (ldap) | `:389` | host:port | `:1389` | |
| `GOGOMAIL_LDAPS_ADDR` | O | _빈 값_ | host:port | `:1636` | LDAP TLS 파일 필요. |
| `GOGOMAIL_LDAP_COMPANY_ID` | R (ldap) | _빈 값_ | UUID | _company id_ | |
| `GOGOMAIL_LDAP_BASE_DOMAIN` | R (ldap) | _빈 값_ | string | `example.com` | |

### 5.8 워커 / 이벤트

| Env | 범위 | 기본 | 형식 | 예시 | 비고 |
|---|---|---|---|---|---|
| `GOGOMAIL_OUTBOX_RELAY_BATCH_SIZE` | O | `100` | int >0 | `200` | |
| `GOGOMAIL_OUTBOX_RELAY_POLL_INTERVAL` | O | `1s` | duration | `1s` | |
| `GOGOMAIL_OUTBOX_RELAY_MAX_ATTEMPTS` | O | (양수) | int >0 | `10` | |
| `GOGOMAIL_DELIVERY_STREAM` | O | `delivery.event` | string | `delivery.event` | |
| `GOGOMAIL_DELIVERY_CONSUMER_GROUP` | O | `gogomail.delivery-worker` | string | _기본_ | |
| `GOGOMAIL_DELIVERY_CONSUMER_COUNT` | O | 양수 | int >0 | `4` | |
| `GOGOMAIL_DELIVERY_THROTTLE_BACKEND` | O | `local` | enum `local/redis` | `redis` | 클러스터 throttle. |
| `GOGOMAIL_SEARCH_INDEX_BACKEND` | O | `disabled` | enum `disabled/postgres/opensearch` | `opensearch` | |
| `GOGOMAIL_SEARCH_INDEX_OPENSEARCH_ENDPOINT` | R (opensearch) | _빈 값_ | URL | `https://opensearch:9200` | |
| `GOGOMAIL_SEARCH_INDEX_OPENSEARCH_INDEX` | R (opensearch) | _빈 값_ | string | `gogomail-mail` | |
| `GOGOMAIL_PUSH_NOTIFICATION_BACKEND` | O | `none` | enum `none/slog/webhook` | `webhook` | APNs/WebPush env 추가. |
| `GOGOMAIL_APNS_KEY_ID` / `_TEAM_ID` / `_PRIVATE_KEY` / `_BUNDLE_ID` | O (APNs) | _빈 값_ | string | _APNs_ | |
| `GOGOMAIL_WEBPUSH_VAPID_PUBLIC_KEY` / `_PRIVATE_KEY` / `_CONTACT_EMAIL` | O (WebPush) | _빈 값_ | string | _VAPID_ | |
| `GOGOMAIL_API_METERING_BACKEND` | O | `none` | enum `none/slog/outbox` | `outbox` | |
| `GOGOMAIL_EVENT_STREAM` | O | _기본_ | string | `gogomail.event` | |
| `GOGOMAIL_EVENT_CONSUMER_GROUP` | O | _기본_ | string | _기본_ | |
| `GOGOMAIL_EVENT_CONSUMER_COUNT` | O | 양수 | int >0 | `4` | |

### 5.9 관찰 가능성 / 레이트 리밋 / 기타

| Env | 범위 | 기본 | 형식 | 예시 | 비고 |
|---|---|---|---|---|---|
| `GOGOMAIL_METRICS_BACKEND` | O | `none` | enum `none/slog/prometheus` | `prometheus` | |
| `GOGOMAIL_METRICS_ADDR` | O | _빈 값_ | host:port | `:9090` | prometheus 일 때 필수. |
| `GOGOMAIL_MAIL_MUTATION_RATELIMIT_BACKEND` | O | `none` | enum `none/redis` | `redis` | |
| `GOGOMAIL_MAIL_MUTATION_RATELIMIT_PER_MINUTE` | O | `300` | int >0 | `300` | |
| `GOGOMAIL_DRIVE_SHARE_RATELIMIT_PER_MINUTE` | O | `120` | int >0 | `120` | |
| `GOGOMAIL_ATTACHMENT_SCAN_BACKEND` | O | `none` | enum `none/webhook/clamav` | `clamav` | |
| `GOGOMAIL_ATTACHMENT_SCAN_CLAMAV_ADDR` | R (clamav) | `127.0.0.1:3310` | host:port | `clamav:3310` | |
| `GOGOMAIL_AUTO_PURGE_ENABLED` | O | `false` | bool | `false` | 휴지통 자동 정리. |
| `GOGOMAIL_HTTP_MAX_HEADER_BYTES` | O | `65536` | int 4096..1048576 | `65536` | |

전체 목록: `grep envOrDefault internal/config/config.go`.
검증기 동작: `internal/config/validate.go`.

---

## 6. Compose 레시피

각 레시피는 출발점입니다. `docker compose up -d --scale <service>=N`
으로 복제본을 조정합니다.

### 6.0 로컬 개발 / 에이전트 운영

로컬 개발, E2E 검증, 에이전트 기반 서비스 운영에는
[`docker-compose.dev.yml`](docker-compose.dev.yml)을 사용하세요. 이 파일은
앱만 띄우는 얇은 compose가 아니라 단일 호스트용 완전한 기본 스택입니다.

| 그룹 | 서비스 |
|---|---|
| 코어 인프라 | postgres, redis, minio, minio-init, clamav |
| 앱/런타임 | backend hot reload, event-worker, outbox-relay, delivery-worker, edge-mta |
| 검색 | opensearch, search-index-worker |
| 모니터링/로그 | prometheus, loki, promtail, grafana |

```bash
docker compose -f docker/docker-compose.dev.yml up -d
docker compose -f docker/docker-compose.dev.yml ps
docker compose -f docker/docker-compose.dev.yml logs -f backend
docker compose -f docker/docker-compose.dev.yml down -v
```

기본 로컬 엔드포인트:

| 엔드포인트 | 용도 |
|---|---|
| `http://localhost:8080` | Backend HTTP API + admin console |
| `localhost:2525` | Edge-MTA SMTP 테스트 인입 |
| `http://localhost:9200` | OpenSearch REST 점검 |
| `http://localhost:9090` | Prometheus |
| `http://localhost:3100` | Loki |
| `http://localhost:3000` | Grafana (`admin` / `admin`, `GRAFANA_PASSWORD`로 변경) |
| `localhost:15432` | Postgres 직접 접속 |
| `localhost:16379` | Redis 직접 접속 |
| `http://localhost:19000` | MinIO S3 API |
| `http://localhost:19001` | MinIO console |

dev 스택은 API 상태, 큐 워커, OpenSearch 인덱싱, 메트릭, 로그, 대시보드가
overlay 없이 함께 올라오므로 자동화 에이전트의 기본 실행 환경입니다. 독립
[`docker-compose.monitoring.yml`](docker-compose.monitoring.yml)과
[`docker-compose.opensearch.yml`](docker-compose.opensearch.yml)은 운영형
분리 스택 또는 다른 compose 토폴로지에 관측/검색 계층만 붙일 때 사용합니다.

### 6.1 Pattern A — 단일 노드 (데모 / 매우 소형)

**토폴로지**

```
                 +---------------------------------+
   Internet ---> | host: docker-compose.small.yml  |
                 |  - nginx (TLS 종료)              |
                 |  - gogomail (all-in-one)         |
                 |  - postgres 16                   |
                 |  - redis 7                       |
                 |  - minio (단일 노드)             |
                 +---------------------------------+
```

**Compose** — [`docker-compose.small.yml`](docker-compose.small.yml) 그대로
사용. `.env` 필수 수정:

```bash
GOGOMAIL_ENV=production
GOGOMAIL_AUTH_JWT_SECRET=$(openssl rand -base64 32)
GOGOMAIL_ADMIN_TOKEN=$(openssl rand -base64 32)
GOGOMAIL_PUBLIC_BASE_URL=https://mail.example.com
GOGOMAIL_DATABASE_URL=postgres://gogomail:STRONGPW@postgres:5432/gogomail?sslmode=require
GOGOMAIL_REDIS_PASSWORD=$(openssl rand -base64 24)
GOGOMAIL_STORAGE_BACKEND=minio
GOGOMAIL_STORAGE_S3_ENDPOINT=https://minio:9000
GOGOMAIL_STORAGE_S3_FORCE_PATH_STYLE=true
GOGOMAIL_FARM_COORDINATOR_BACKEND=redis
GOGOMAIL_SMTP_DOMAIN=mail.example.com
GOGOMAIL_DELIVERY_SMTP_HELLO=mail.example.com
POSTGRES_PASSWORD=STRONGPW
MINIO_ROOT_PASSWORD=$(openssl rand -base64 24)
```

End-to-end 운영을 위해 워커 모드를 별도 서비스로 추가:

```yaml
  outbox-relay:
    image: ${BACKEND_IMAGE:-gogomail:latest}
    restart: always
    depends_on:
      postgres: { condition: service_healthy }
      redis:    { condition: service_healthy }
    environment:
      APP_MODE: outbox-relay
    env_file: .env
    networks: [gogomail]

  delivery-worker:
    image: ${BACKEND_IMAGE:-gogomail:latest}
    restart: always
    depends_on:
      postgres: { condition: service_healthy }
      redis:    { condition: service_healthy }
    environment:
      APP_MODE: delivery-worker
    env_file: .env
    networks: [gogomail]
```

**nginx** — [`nginx-single.conf`](nginx-single.conf) 가 HTTP를 처리.
SMTP/IMAP는 backend 컨테이너에서 직접 노출
(`backend.ports: ["25:2525", "587:2587", "143:1143", "993:1993"]`)
하거나 HAProxy 추가.

**검증**:

```bash
docker compose -f docker-compose.small.yml up -d
docker compose logs -f backend
docker compose exec backend gogomail -migrate
curl -k https://localhost/health/ready
```

### 6.2 Pattern B — 소형 (~1k 메일박스)

**토폴로지**

```
                 (TLS @ :443/:25/:587/:993)
                            |
                     +------+------+
                     |    nginx    |
                     +------+------+
                            |
              +-------------+-------------+
              |                           |
        +-----+-----+               +-----+-----+
        |  app-1    |               |  app-2    |    (all-in-one)
        +-----------+               +-----------+
                            |
                     +------+------+
                     |  worker-1   |     (outbox-relay + delivery-worker +
                     +-------------+      cleanup 워커 + batch-worker)
                            |
        +-------------------+--------------------+
        |              |              |
   +----+-----+   +----+----+   +-----+----+
   | Postgres |   | Redis   |   | S3/MinIO |   (관리형 또는 자체)
   +----------+   +---------+   +----------+
```

`docker-compose.small.yml`을 확장: `backend`를 `backend-1`, `backend-2`로
복제 (`INSTANCE_ID` 구분), nginx LB upstream에 추가, §6.1의 워커 서비스
포함. 가능하면 관리형 PG/Redis 사용.

`.env` 추가:

```bash
GOGOMAIL_RATELIMIT_BACKEND=redis
GOGOMAIL_BACKPRESSURE_BACKEND=redis
GOGOMAIL_MAIL_MUTATION_RATELIMIT_BACKEND=redis
GOGOMAIL_DRIVE_SHARE_RATELIMIT_BACKEND=redis
GOGOMAIL_DELIVERY_THROTTLE_BACKEND=redis
GOGOMAIL_METRICS_BACKEND=prometheus
GOGOMAIL_METRICS_ADDR=:9090
GOGOMAIL_DKIM_ENABLED=true
GOGOMAIL_SMTP_AUTH_VERIFICATION_ENABLED=true
```

### 6.3 Pattern C — 중형 (~50k 메일박스)

[`docker-compose.medium.yml`](docker-compose.medium.yml)이 출발점.
PG primary+replica, Redis Sentinel, MinIO 3노드, Prometheus, backend 2개
포함.

**필수 커스터마이즈**:

1. **모드 분리**: `all-in-one` 2개 대신 역할별 Deployment:
   - `edge` ×3 (`APP_MODE=edge-mta`)
   - `outbound` ×2
   - `mail-api` ×3 / `admin-api` ×2 / `auth` ×2
   - `imap` ×3 / `pop3` ×2
   - `caldav` ×2 / `carddav` ×2 / `webdav` ×2 / `ldap` ×2
   - `outbox-relay` ×2 (싱글톤)
   - `delivery-worker` ×3
   - `search-index-worker` ×2 ([`docker-compose.opensearch.yml`](docker-compose.opensearch.yml)
     또는 관리형 OpenSearch/Elasticsearch 엔드포인트 필요)
   - `push-notification-worker` ×2 / `api-metering-worker` ×2
   - cleanup / retention / batch 각 ×1 + cold-standby 1

2. **SMTP 분산**: nginx 대신 HAProxy L4. 예시:

```haproxy
frontend fe_smtp_25
    bind *:25
    mode tcp
    default_backend be_edge
backend be_edge
    mode tcp
    balance leastconn
    server edge-1 edge-1:2525 check
    server edge-2 edge-2:2525 check
    server edge-3 edge-3:2525 check
```

3. **nginx (HTTP)** — [`nginx-backend.conf`](nginx-backend.conf) 멀티
upstream 지원. HSTS / rate-limit 디렉티브 추가:

```nginx
add_header Strict-Transport-Security "max-age=63072000; includeSubDomains; preload" always;
limit_req_zone $binary_remote_addr zone=api:10m rate=10r/s;
```

### 6.4 Pattern D — 대형 (multi-DC, 100k+ 메일박스)

[`docker-compose.large.yml`](docker-compose.large.yml) — 단일 DC 참조
스택 (PG 3노드 + etcd, Redis 3노드 cluster, MinIO 6노드, ELK +
Prometheus + Grafana, HAProxy).

실무에서는 Kubernetes로 운영. compose의 각 `service`를 `Deployment` +
`Service`로 변환:

- DC당 모드별 Deployment (24개까지)
- `HorizontalPodAutoscaler`:
  - `edge-mta`: 초당 SMTP 세션
  - `mail-api`/`admin-api`: HTTP RPS
  - `imap`: 활성 연결 수
  - 워커: 각자의 Redis 스트림 lag
- 싱글톤: 글로벌 2-3 복제본; PG primary DC에 핀.
- `outbox-relay`는 노드간 anti-affinity, DC 단위로 의도적으로 하나만.

Cross-DC 토폴로지:

```
                  Geo DNS / Anycast
                          |
        +-----------------+-----------------+
        |                                   |
   +----+----+                         +----+----+
   |  DC-A    |  <-- WAL 스트림 -->     |  DC-B    |
   | (active) |  <-- redis sentinel -->| (standby)|
   +----+----+                         +----+----+
        \                                   /
         \---- S3 cross-region 복제 ------/
```

---

## 7. 네트워크 노출 매트릭스

```
PUBLIC (인터넷):
  25/tcp   SMTP inbound (edge-mta)
  465/tcp  SMTPS (outbound-mta implicit TLS)
  587/tcp  Submission (outbound-mta STARTTLS)
  443/tcp  HTTPS — webmail, 관리자, mail-api, admin-api, auth-server
  993/tcp  IMAPS (imap)
  995/tcp  POP3S (pop3) — 불필요하면 VPN 제한
  636/tcp  LDAPS — 원격 LDAP 필요 시

INTERNAL (클러스터 / VPN 전용):
  80/tcp   HTTP (LB → backend)
  143/tcp  IMAP STARTTLS
  110/tcp  POP3 STARTTLS
  389/tcp  LDAP (prod에서는 LDAPS)
  2525/tcp gogomail SMTP 리스너 (25 매핑)
  2526/tcp inbound-mta (trusted relay 전용)
  2587/tcp submission 리스너 (587 매핑)
  1143/tcp IMAP 리스너 (143 매핑)
  1110/tcp POP3 리스너 (110 매핑)
  1389/tcp LDAP 리스너 (389 매핑)
  1995/tcp POP3S 리스너 (995 매핑)
  8080/tcp gogomail HTTP API
  8081/tcp CalDAV / 8082/tcp CardDAV / 8083/tcp WebDAV
  9000/tcp MinIO API / 9001/tcp MinIO 콘솔
  9090/tcp Prometheus 메트릭
  5432/tcp Postgres / 6379/tcp Redis / 26379 Sentinel
  9200/tcp OpenSearch / 3310/tcp ClamAV
```

방화벽: 기본 deny, PUBLIC만 허용. INTERNAL은 gogomail 노드에서만 접근.
PG/Redis/MinIO는 백엔드 SG로 제한.

---

## 8. DNS 설정

도메인 `example.com`, 메일 호스트 `mail.example.com` 기준:

| 타입 | 이름 | 값 | 필수 | 비고 |
|---|---|---|---|---|
| A / AAAA | `mail.example.com` | _공개 IP_ | 필수 | `SMTP_DOMAIN`, `DELIVERY_SMTP_HELLO` 사용. |
| A / AAAA | `webmail.example.com` | _공개 IP_ | 필수 | 웹메일/mail-api. |
| A / AAAA | `admin.example.com` | _공개 IP_ | 필수 | 관리자 콘솔. |
| A / AAAA | `autodiscover.example.com` | _공개 IP_ | 권장 | Outlook autodiscover. |
| MX | `example.com` | `10 mail.example.com.` | 필수 | 수신 메일. |
| TXT (SPF) | `example.com` | `v=spf1 mx -all` | 필수 | |
| TXT (DKIM) | `default._domainkey.example.com` | `v=DKIM1; k=rsa; p=…` | 필수 | `gogomail admin dkim:print --domain example.com` 출력 사용. 선택자 기본 `default`. |
| TXT (DMARC) | `_dmarc.example.com` | `v=DMARC1; p=reject; rua=mailto:dmarc@example.com; adkim=s; aspf=s` | 필수 | 테스트 동안 `p=quarantine`. |
| TXT (MTA-STS) | `_mta-sts.example.com` | `v=STSv1; id=20260101000000;` | 권장 | `https://mta-sts.example.com/.well-known/mta-sts.txt` (version: STSv1, mode: enforce, mx: mail.example.com) 게시. |
| TXT (TLS-RPT) | `_smtp._tls.example.com` | `v=TLSRPTv1; rua=mailto:tlsrpt@example.com` | 권장 | |
| TLSA | `_25._tcp.mail.example.com` | _3 1 1 \<sha256(spki)\>_ | 선택 | DANE 활성화. |
| CAA | `example.com` | `0 issue "letsencrypt.org"` | 권장 | 발급 CA 제한. |

**역방향 DNS (PTR)**: 메일 발신 공개 IP는 `mail.example.com`으로 PTR
해야 함. PTR 미설정 시 Gmail/Outlook 거부/스팸 처리 위험.

---

## 9. TLS / 인증서

### 9.1 ACME (Let's Encrypt) + Caddy

단일 노드/소형에는 nginx 대신 Caddy:

```caddyfile
mail.example.com, webmail.example.com, admin.example.com {
    reverse_proxy backend:8080
    encode gzip zstd
    header Strict-Transport-Security "max-age=63072000; includeSubDomains; preload"
}
```

SMTP/IMAP TLS는 Caddy 인증서를 컨테이너에 마운트:

```yaml
backend:
  volumes:
    - caddy-data:/data:ro
  environment:
    GOGOMAIL_SMTP_TLS_CERT_FILE: /data/caddy/certificates/.../mail.example.com.crt
    GOGOMAIL_SMTP_TLS_KEY_FILE:  /data/caddy/certificates/.../mail.example.com.key
    GOGOMAIL_IMAP_TLS_CERT_FILE: /data/caddy/certificates/.../mail.example.com.crt
    GOGOMAIL_IMAP_TLS_KEY_FILE:  /data/caddy/certificates/.../mail.example.com.key
```

### 9.2 Certbot + nginx

```bash
certbot certonly --standalone -d mail.example.com -d webmail.example.com -d admin.example.com
# /etc/letsencrypt/live/mail.example.com/
```

`/etc/letsencrypt`를 nginx와 backend에 read-only로 마운트.

### 9.3 nginx 인증서 경로

[`nginx-backend.conf`](nginx-backend.conf) 는 `/etc/nginx/certs/`를
사용합니다. 컨벤션:

```
/etc/nginx/certs/
  fullchain.pem
  privkey.pem
  dhparam.pem (선택, 2048-bit)
```

### 9.4 IMAP/POP3 mTLS (선택)

기본 비활성화. 사이드카 (envoy 또는 nginx stream) 에서 `ssl_verify_client
on` 으로 종료한 뒤 loopback으로 평문 전달; 클라이언트 인증서 → SASL
매핑은 커스텀 milter 필요 (기본 미포함).

---

## 10. 초기 설정

신규 배포 부트스트랩 절차:

```bash
# 1. 클론
git clone https://github.com/gogomail/gogomail.git
cd gogomail/docker

# 2. .env 구성
cp .env.example .env
# 최소 수정:
#   GOGOMAIL_ENV=production
#   GOGOMAIL_PUBLIC_BASE_URL=https://mail.example.com
#   POSTGRES_PASSWORD, MINIO_ROOT_PASSWORD, REDIS_PASSWORD

# 3. 시크릿 생성
echo "GOGOMAIL_AUTH_JWT_SECRET=$(openssl rand -base64 32)" >> .env
echo "GOGOMAIL_ADMIN_TOKEN=$(openssl rand -base64 32)" >> .env

# 4. 인프라 먼저 기동
docker compose -f docker-compose.small.yml up -d postgres redis minio minio-init

# 5. 마이그레이션
docker compose run --rm backend gogomail -migrate

# 6. 설정 검증
docker compose run --rm backend gogomail -validate-config

# 7. 백엔드 + LB
docker compose -f docker-compose.small.yml up -d backend nginx

# 8. 부트스트랩 super-admin 생성
docker compose exec backend gogomail admin create-admin \
    --email admin@example.com --scope super-admin

# 9. 첫 DKIM 키 생성
docker compose exec backend gogomail admin dkim:rotate \
    --domain example.com --selector default
# 출력된 TXT 레코드를 default._domainkey.example.com 에 게시

# 10. 스모크 테스트
curl -fsS https://mail.example.com/health/ready
swaks --to test@example.com --from probe@external.tld \
    --server mail.example.com --tls
```

---

## 11. 운영

런북은 [`docs/OPERATIONS.md`](../docs/OPERATIONS.md) 참조.

| 주제 | 실무 |
|---|---|
| 백업 | 야간 `pg_dump` + WAL 아카이빙 (S3). 메일 블롭 버킷에 versioning 활성화. |
| 복구 드릴 | 분기. `gogomail tools restore-rehearsal` 가 `GOGOMAIL_RESTORE_REHEARSAL_DATABASE_URL`에 비파괴 복구 실행. |
| 스키마 마이그레이션 | `docker compose run --rm backend gogomail -migrate`. 전진 전용, 멱등. |
| 스케일 아웃 트리거 | `delivery.event` lag > 30s 5분 → `delivery-worker` 증설. `mail-api` p95 > 500ms → API 복제본 증설. |
| 장애: PG primary | replica 승격; 옛 primary에 핀된 싱글톤 재시작. |
| 장애: Redis | Sentinel/Cluster 자동 failover. 복구 시 outbox 자동 배출. |
| 장애: S3 | 메타데이터 흐름 유지; 본문 가져오기/업로드만 실패. |
| 시크릿 회전 | `GOGOMAIL_AUTH_JWT_SECRET` — 한 refresh 토큰 수명 동안 신구 둘 다 배포 후 옛 키 제거. |
| 로그 보존 | slog JSON → stdout, Docker 로깅 드라이버로 전달. 시크릿 키(`password`, `token`, `secret`, `key`, `private_key`)는 자동 마스킹. |

---

## 12. AI 에이전트용 지침

**사용자가 gogomail용 docker-compose 또는 k8s 매니페스트 생성을 요청
하면 다음 절차를 결정적으로 따르세요.**

### 절차

1. **입력 수집**:
   - 대략적 메일박스 수
   - 일일 메일 수
   - HA 요구 (single AZ / multi-AZ / multi-DC)
   - 관리형 서비스 사용 (PG, Redis, S3?)
   - 공개 hostname
   - OpenSearch 검색 + push 알림 필요 여부
2. **§3 의사결정 트리로 패턴 선택**.
3. **§6 매칭 레시피에서 시작**:
   - Pattern A → [`docker-compose.small.yml`](docker-compose.small.yml)
   - Pattern B → `small.yml` 파생, `backend` ×2 + `outbox-relay` +
     `delivery-worker` 추가
   - Pattern C → [`docker-compose.medium.yml`](docker-compose.medium.yml)
     + §6.3의 역할 분리
   - Pattern D → [`docker-compose.large.yml`](docker-compose.large.yml)
     을 DC 템플릿으로 사용, 운영은 k8s
4. **§5 env 변수 구성**. 프로덕션 최소 셋:
   ```
   GOGOMAIL_ENV=production
   GOGOMAIL_AUTH_JWT_SECRET=<openssl rand -base64 32>       # ≥32 bytes
   GOGOMAIL_ADMIN_TOKEN=<openssl rand -base64 32>           # ≥32 bytes
   GOGOMAIL_REDIS_PASSWORD=<random>                         # non-empty
   GOGOMAIL_FARM_COORDINATOR_BACKEND=redis
   GOGOMAIL_DATABASE_URL=postgres://…?sslmode=require       # disable 금지
   GOGOMAIL_STORAGE_S3_ENDPOINT=https://…                    # HTTPS
   GOGOMAIL_STORAGE_S3_INSECURE_SKIP_VERIFY=false
   GOGOMAIL_PUBLIC_BASE_URL=https://<hostname>              # https, no localhost
   GOGOMAIL_SMTP_DOMAIN=<public hostname>                   # localhost 금지
   GOGOMAIL_DELIVERY_SMTP_HELLO=<public hostname>           # localhost 금지
   GOGOMAIL_SUBMISSION_ALLOW_INSECURE_AUTH=false
   GOGOMAIL_IMAP_ALLOW_INSECURE_AUTH=false
   GOGOMAIL_CALDAV_ALLOW_INSECURE_AUTH=false
   GOGOMAIL_CARDDAV_ALLOW_INSECURE_AUTH=false
   GOGOMAIL_METRICS_BACKEND=prometheus
   GOGOMAIL_METRICS_ADDR=:9090
   GOGOMAIL_RATELIMIT_BACKEND=redis
   GOGOMAIL_BACKPRESSURE_BACKEND=redis
   GOGOMAIL_DKIM_ENABLED=true
   GOGOMAIL_SMTP_AUTH_VERIFICATION_ENABLED=true
   GOGOMAIL_SMTP_DMARC_ENFORCEMENT=reject
   ```
5. **§8에 따라 DNS 구성**: A/AAAA, MX, SPF, DKIM, DMARC, PTR 최소 포함.
6. **§6 + §9에 따라 LB 구성**. 항상:
   - LB에서 TLS 종료
   - HSTS `max-age=63072000; includeSubDomains; preload` 전송
   - `X-Forwarded-Proto`, `X-Real-IP` 헤더 설정
   - LB CIDR을 `GOGOMAIL_CALDAV_TRUSTED_PROXIES`,
     `GOGOMAIL_CARDDAV_TRUSTED_PROXIES`에 추가
7. **완료 전 검증**:
   ```bash
   docker compose run --rm backend gogomail -validate-config
   docker compose up -d
   docker compose run --rm backend gogomail -migrate
   curl -fsS https://<hostname>/health/ready
   ```
8. **사용자에게 보고**:
   - 선택 패턴과 근거 (§3 인용)
   - 레시피 대비 변경점과 이유
   - 남은 수동 작업 (DNS 게시, 인증서 발급, super-admin 생성)

### 제약 (하드 룰)

- 다음을 모두 만족하지 않으면 **절대** `GOGOMAIL_ENV=production`을
  내보내지 마세요: `JWT_SECRET ≥ 32 bytes`, 비어있지 않은 `ADMIN_TOKEN`,
  비어있지 않은 `REDIS_PASSWORD`, `FARM_COORDINATOR_BACKEND=redis`,
  PG DSN `sslmode=require`, HTTPS S3 엔드포인트,
  `INSECURE_SKIP_VERIFY=false`. 검증기가 기동을 거부합니다.
- Postgres/Redis/MinIO API 포트는 **절대** 인터넷에 노출하지 마세요.
- IMAP/POP3/SMTP를 평문으로만 인터넷에 매핑하지 마세요 — 항상 STARTTLS
  또는 implicit TLS와 함께.
- prod에서 `*_ALLOW_INSECURE_AUTH=true` 절대 금지.
- prod에서 `GOGOMAIL_SMTP_DOMAIN` / `GOGOMAIL_DELIVERY_SMTP_HELLO`를
  `localhost`/`127.0.0.1`/unspecified로 절대 설정 금지.
- 동일 farm에서 `outbox-relay`를 2개 이상 활성으로 두지 마세요 — 2
  복제본은 failover 전용, lock으로 선출.
- Postgres/Redis/MinIO 데이터는 **항상** named volume 사용 (운영자가
  명시적으로 호스트 경로 요청하지 않는 한).

### 참조 요약

- Compose 레시피: 위 §6 + `docker-compose.{dev,small,medium,large,scale,monitoring,opensearch}.yml`
- Env 변수: 위 §5, 출처 `internal/config/config.go`
- 모드 참조: 위 §4, 출처 [`docs/MODES.md`](../docs/MODES.md) +
  `internal/app/mode.go`
- 검증기: `internal/config/validate.go`. 본 문서는 검증기 동작과 일치
  해야 합니다.
