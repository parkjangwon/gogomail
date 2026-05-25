# GoGoMail — Kubernetes Deployment

Minimal production-ready manifests for deploying GoGoMail on Kubernetes.

## File overview

| File | Purpose |
|------|---------|
| `namespace.yaml` | `gogomail` namespace |
| `configmap.yaml` | Non-secret runtime configuration (ports, flags, Redis addr, …) |
| `secret.yaml` | Sensitive values (DB URL, JWT secret, DM master key, S3 creds) |
| `deployment.yaml` | 2-replica Deployment with rolling update, readiness/liveness probes, non-root security context |
| `service.yaml` | ClusterIP Service (HTTP/SMTP/IMAP) + headless Service for IMAP sticky sessions |
| `hpa.yaml` | HorizontalPodAutoscaler (2–10 replicas, CPU 70% / memory 80%) |
| `pdb.yaml` | PodDisruptionBudget (minAvailable: 1) |
| `ingress.yaml` | NGINX Ingress for the HTTP API; TLS via cert-manager (commented out) |

## Quick start

```bash
# 1. Edit secret.yaml — replace every CHANGEME value.
#    In production, use Sealed Secrets or External Secrets Operator instead.

# 2. Apply in order
kubectl apply -f k8s/namespace.yaml
kubectl apply -f k8s/configmap.yaml
kubectl apply -f k8s/secret.yaml
kubectl apply -f k8s/deployment.yaml
kubectl apply -f k8s/service.yaml
kubectl apply -f k8s/hpa.yaml
kubectl apply -f k8s/pdb.yaml
kubectl apply -f k8s/ingress.yaml
```

Or apply the whole directory at once:

```bash
kubectl apply -f k8s/
```

## Multi-mode deployment

GoGoMail runs as a single binary with 24 runtime modes (api, smtp, imap, pop3, …).
The `deployment.yaml` defaults to `args: ["api"]`. For a split deployment, create
one Deployment per mode with the appropriate `args` and ports:

```yaml
# SMTP Deployment
args: ["smtp"]
ports:
  - containerPort: 2525

# IMAP Deployment
args: ["imap"]
ports:
  - containerPort: 1143
```

## Required secrets

| Env var | Description |
|---------|-------------|
| `GOGOMAIL_DATABASE_URL` | PostgreSQL connection URL (`postgres://user:pass@host:5432/db?sslmode=require`) |
| `GOGOMAIL_AUTH_JWT_SECRET` | JWT signing secret (≥ 32 random bytes) |
| `GOGOMAIL_ADMIN_TOKEN` | Bearer token for the admin API |
| `GOGOMAIL_DM_MASTER_KEY` | AES-256 master key for DM encryption (32-byte hex: `openssl rand -hex 32`) |
| `GOGOMAIL_STORAGE_S3_*` | S3-compatible object storage credentials |

## TLS for IMAP/SMTP

For IMAPS/SMTPS, mount a TLS Secret as a volume and set:

```yaml
env:
  - name: GOGOMAIL_IMAP_TLS_CERT_FILE
    value: /etc/tls/tls.crt
  - name: GOGOMAIL_IMAP_TLS_KEY_FILE
    value: /etc/tls/tls.key
volumes:
  - name: tls
    secret:
      secretName: gogomail-tls
volumeMounts:
  - name: tls
    mountPath: /etc/tls
    readOnly: true
```

## Observability

GoGoMail exposes a `/metrics` Prometheus endpoint on the HTTP port.
Add a `ServiceMonitor` (Prometheus Operator) or a Prometheus scrape config
pointing at port `8080` path `/metrics`.
