# Waitlist Operations

The waitlist service stores signups in a PostgreSQL database bundled in the same Helm release. Both run in the `ownpulse` namespace.

## Querying signups

Connect to the postgres pod and run psql:

```bash
ssh root@ownpulse-us "kubectl exec -n ownpulse waitlist-ownpulse-waitlist-postgres-0 -- psql -U waitlist -d waitlist -c 'SELECT id, email, name, persona, created_at FROM waitlist ORDER BY created_at DESC;'"
```

### Count by persona

```bash
ssh root@ownpulse-us "kubectl exec -n ownpulse waitlist-ownpulse-waitlist-postgres-0 -- psql -U waitlist -d waitlist -c 'SELECT persona, count(*) FROM waitlist GROUP BY persona ORDER BY count DESC;'"
```

### Total count

```bash
ssh root@ownpulse-us "kubectl exec -n ownpulse waitlist-ownpulse-waitlist-postgres-0 -- psql -U waitlist -d waitlist -c 'SELECT count(*) FROM waitlist;'"
```

### Export as CSV

```bash
ssh root@ownpulse-us "kubectl exec -n ownpulse waitlist-ownpulse-waitlist-postgres-0 -- psql -U waitlist -d waitlist -c \"COPY (SELECT email, name, persona, created_at FROM waitlist ORDER BY created_at) TO STDOUT WITH CSV HEADER\"" > waitlist-export.csv
```

## Monitoring

The waitlist service exposes Prometheus metrics at `/metrics`:

- `waitlist_signups_total{status}` — signup counter (created/error)
- `waitlist_http_requests_total{method,path,status_code}` — request counter
- `waitlist_http_request_duration_seconds{method,path}` — latency histogram

A Grafana dashboard is available under "Waitlist" (pending infra PR).

## Service health

```bash
ssh root@ownpulse-us "kubectl exec -n ownpulse deploy/waitlist-ownpulse-waitlist -- wget -qO- http://localhost:8080/healthz"
```

## Architecture

- **waitlist service** — Go binary, listens on :8080, handles `POST /api/waitlist` and `GET /healthz`
- **postgres** — PostgreSQL 16, bundled in the Helm chart as a StatefulSet with 1Gi PVC
- **nginx proxy** — the marketing site's nginx proxies `/api/waitlist` to the waitlist service
