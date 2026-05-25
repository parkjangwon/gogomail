# gogomail Helm Chart

Helm chart for [gogomail](https://github.com/gogomail/gogomail) — a self-hosted email platform written in Go (single binary, multi-mode: HTTP API, SMTP, IMAP, POP3, CalDAV, CardDAV, WebDAV, LDAP).

## Prerequisites

- Helm 3.x
- Kubernetes 1.25+
- External **PostgreSQL** database (connection URL required)
- External **Redis** instance (for queuing and caching)
- S3-compatible object storage (for email attachments)

## Quick Start

```bash
helm install gogomail ./helm/gogomail \
  --set secrets.GOGOMAIL_DATABASE_URL="postgres://gogomail:pass@postgres:5432/gogomail?sslmode=require" \
  --set secrets.GOGOMAIL_AUTH_JWT_SECRET="$(openssl rand -base64 32)" \
  --set secrets.GOGOMAIL_ADMIN_TOKEN="$(openssl rand -hex 16)" \
  --set secrets.GOGOMAIL_DM_MASTER_KEY="$(openssl rand -hex 32)" \
  --set secrets.GOGOMAIL_STORAGE_S3_ENDPOINT="https://s3.amazonaws.com" \
  --set secrets.GOGOMAIL_STORAGE_S3_BUCKET="my-gogomail-bucket" \
  --set secrets.GOGOMAIL_STORAGE_S3_ACCESS_KEY_ID="AKIAIOSFODNN7EXAMPLE" \
  --set secrets.GOGOMAIL_STORAGE_S3_SECRET_ACCESS_KEY="wJalrXUtnFEMI..."
```

To enable ingress:

```bash
helm install gogomail ./helm/gogomail \
  --set ingress.enabled=true \
  --set ingress.hosts[0].host=mail.example.com \
  --set ingress.hosts[0].paths[0].path=/ \
  --set ingress.hosts[0].paths[0].pathType=Prefix \
  ... (secrets as above)
```

To enable autoscaling:

```bash
helm upgrade gogomail ./helm/gogomail \
  --set autoscaling.enabled=true \
  --set autoscaling.minReplicas=2 \
  --set autoscaling.maxReplicas=10
```

## Values Reference

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `replicaCount` | int | `1` | Number of pod replicas (PDB only active when > 1) |
| `image.repository` | string | `ghcr.io/gogomail/gogomail` | Container image repository |
| `image.tag` | string | `"latest"` | Image tag; defaults to `appVersion` if empty |
| `image.pullPolicy` | string | `IfNotPresent` | Image pull policy |
| `imagePullSecrets` | list | `[]` | Image pull secrets |
| `nameOverride` | string | `""` | Override chart name portion of resource names |
| `fullnameOverride` | string | `""` | Override full resource name |
| `serviceAccount.create` | bool | `true` | Create a ServiceAccount |
| `serviceAccount.annotations` | object | `{}` | ServiceAccount annotations (e.g. for IRSA/Workload Identity) |
| `serviceAccount.name` | string | `""` | ServiceAccount name override |
| `podAnnotations` | object | `{}` | Extra pod annotations |
| `podSecurityContext` | object | runAsNonRoot/uid 1000 | Pod-level security context |
| `securityContext` | object | no privilege escalation, read-only FS, drop ALL caps | Container security context |
| `service.type` | string | `ClusterIP` | Service type |
| `service.ports.http.port` | int | `80` | HTTP service port |
| `service.ports.smtp.port` | int | `25` | SMTP service port |
| `service.ports.smtps.port` | int | `465` | SMTPS service port |
| `service.ports.smtpSubmission.port` | int | `587` | SMTP submission service port |
| `service.ports.imap.port` | int | `143` | IMAP service port |
| `service.ports.imaps.port` | int | `993` | IMAPS service port |
| `ingress.enabled` | bool | `false` | Enable Ingress |
| `ingress.className` | string | `"nginx"` | IngressClass name |
| `ingress.annotations` | object | nginx proxy timeouts/body | Ingress annotations |
| `ingress.hosts` | list | `[{host: mail.example.com, paths: [{path: /, pathType: Prefix}]}]` | Ingress host rules |
| `ingress.tls` | list | `[]` | TLS configuration |
| `resources.requests.cpu` | string | `"250m"` | CPU request |
| `resources.requests.memory` | string | `"256Mi"` | Memory request |
| `resources.limits.cpu` | string | `"1000m"` | CPU limit |
| `resources.limits.memory` | string | `"512Mi"` | Memory limit |
| `autoscaling.enabled` | bool | `false` | Enable HPA |
| `autoscaling.minReplicas` | int | `2` | HPA minimum replicas |
| `autoscaling.maxReplicas` | int | `10` | HPA maximum replicas |
| `autoscaling.targetCPUUtilizationPercentage` | int | `70` | HPA CPU target |
| `autoscaling.targetMemoryUtilizationPercentage` | int | `80` | HPA memory target |
| `podDisruptionBudget.minAvailable` | int | `1` | PDB minAvailable (only created when replicaCount > 1) |
| `config.GOGOMAIL_HTTP_ADDR` | string | `":8080"` | HTTP listen address |
| `config.GOGOMAIL_ENV` | string | `"production"` | Runtime environment |
| `config.GOGOMAIL_SMTP_ADDR` | string | `":2525"` | SMTP outbound listen address |
| `config.GOGOMAIL_INBOUND_SMTP_ADDR` | string | `":2526"` | SMTP inbound listen address |
| `config.GOGOMAIL_IMAP_ADDR` | string | `":1143"` | IMAP listen address |
| `config.GOGOMAIL_IMAP_MAX_CONNECTIONS` | string | `"5000"` | IMAP max connections |
| `config.GOGOMAIL_POP3_ADDR` | string | `":1110"` | POP3 listen address |
| `config.GOGOMAIL_CALDAV_ADDR` | string | `":8081"` | CalDAV listen address |
| `config.GOGOMAIL_CARDDAV_ADDR` | string | `":8082"` | CardDAV listen address |
| `config.GOGOMAIL_WEBDAV_ADDR` | string | `":8083"` | WebDAV listen address |
| `config.GOGOMAIL_LDAP_ADDR` | string | `":389"` | LDAP listen address |
| `config.GOGOMAIL_REDIS_ADDR` | string | `"redis:6379"` | Redis address |
| `config.GOGOMAIL_STORAGE_S3_REGION` | string | `"us-east-1"` | S3 region |
| `config.GOGOMAIL_STORAGE_S3_FORCE_PATH_STYLE` | string | `"false"` | S3 path-style addressing |
| `config.GOGOMAIL_SMTP_DMARC_ENFORCEMENT` | string | `"reject"` | DMARC enforcement policy |
| `config.GOGOMAIL_DELIVERY_RATE_LIMIT_ENABLED` | string | `"false"` | Enable delivery rate limiting |
| `config.GOGOMAIL_DELIVERY_DEFAULT_RATE_LIMIT_PER_MINUTE` | string | `"60"` | Default delivery rate limit |
| `secrets.GOGOMAIL_DATABASE_URL` | string | placeholder | PostgreSQL connection URL |
| `secrets.GOGOMAIL_AUTH_JWT_SECRET` | string | placeholder | JWT signing secret (min 32 bytes) |
| `secrets.GOGOMAIL_ADMIN_TOKEN` | string | placeholder | Admin API bearer token |
| `secrets.GOGOMAIL_DM_MASTER_KEY` | string | placeholder | DM encryption master key (32-byte hex) |
| `secrets.GOGOMAIL_STORAGE_S3_ENDPOINT` | string | placeholder | S3-compatible endpoint URL |
| `secrets.GOGOMAIL_STORAGE_S3_BUCKET` | string | placeholder | S3 bucket name |
| `secrets.GOGOMAIL_STORAGE_S3_ACCESS_KEY_ID` | string | placeholder | S3 access key ID |
| `secrets.GOGOMAIL_STORAGE_S3_SECRET_ACCESS_KEY` | string | placeholder | S3 secret access key |
| `persistence.enabled` | bool | `false` | Enable PVC for local storage |
| `persistence.storageClass` | string | `""` | StorageClass for PVC |
| `persistence.size` | string | `"10Gi"` | PVC size |
| `nodeSelector` | object | `{}` | Node selector |
| `tolerations` | list | `[]` | Pod tolerations |
| `affinity` | object | `{}` | Pod affinity rules |

## Upgrading

```bash
helm upgrade gogomail ./helm/gogomail -f my-values.yaml
```

After changing secrets via `kubectl`, roll the deployment to pick them up:

```bash
kubectl rollout restart deployment/gogomail -n <namespace>
```

## Production Recommendations

- **Secrets**: Use [Sealed Secrets](https://github.com/bitnami-labs/sealed-secrets), [External Secrets Operator](https://external-secrets.io/), or [Vault Agent Injector](https://developer.hashicorp.com/vault/docs/platform/k8s/injector) instead of plain Helm values.
- **TLS**: Enable cert-manager with `ingress.annotations["cert-manager.io/cluster-issuer"]` and configure `ingress.tls`.
- **Mail ports**: For direct internet SMTP delivery (port 25), use `service.type=LoadBalancer` with a static IP and configure reverse DNS.
- **Replicas**: Set `replicaCount >= 2` and `autoscaling.enabled=true` for HA. The PDB is only created when `replicaCount > 1`.
- **Resources**: Tune `resources.requests/limits` based on your mail volume.
