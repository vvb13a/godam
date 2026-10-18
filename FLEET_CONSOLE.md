# Fleet / Deployment Console

This document describes what a central godam deployment console must be able to
do, and what every godam instance must expose for it.

The console is **deployment management**, not asset management. It monitors
instances, capacity, backup health, versions, and alerts. It never stores asset
data, asset metadata, SFTP credentials, or consuming-application tokens.

## Deployment model

- One godam instance = one library = one docker compose stack.
- Each instance has its own DB, storage volume, subdomain/domain, and tokens.
- `GODAM_LIBRARY_NAME` is used when the instance creates its library on first
  boot. There is no active-library selection; the instance is the library.
- Instances are independent; the console is optional.
- Instances must continue serving even if the console is down.
- The console uses a pull model: it calls each instance over HTTPS. Instances
  do not need to call the console.

```text
fleet console
  ├── https://project-a.dam.example.com   (godam + imgproxy stack)
  ├── https://project-b.dam.example.com
  └── https://project-c.other-domain.com
```

The console may group instances into organizations for display, but that
grouping is only console metadata. It must not imply shared data, shared auth,
or shared storage.

## Instance prerequisites

Each godam instance must:

- expose `GET /api/health` for public liveness
- expose `GET /api/system/info` and `GET /api/system/metrics` behind a
  per-instance management token
- report its version, schema version, uptime, and library identity
- report storage usage, asset counts, backup status, and background job counts
- report process/container resource usage where available
- remain fully operational without the console

### Management token

Set:

```text
GODAM_MANAGEMENT_TOKEN=<long random secret>
```

Use it as:

```http
Authorization: Bearer <token>
```

or:

```http
X-Godam-Management-Token: <token>
```

Rules:

- one token per instance
- separate from user sessions and library/service tokens
- rotate independently
- never put it in URLs or logs
- if unset, the system endpoints return `404`
- expose the endpoints over HTTPS only; restrict access by firewall or VPN where
  possible

## Instance API contract

### `GET /api/health`

Public liveness. No sensitive data.

```json
{ "status": "ok" }
```

### `GET /api/system/info`

Requires the management token.

```json
{
  "version": "1.2.0",
  "schema_version": 15,
  "started_at": "2026-01-01T10:00:00Z",
  "uptime_seconds": 86400,
  "libraries": [
    { "id": "lib-uuid", "name": "Website" }
  ]
}
```

In the single-library deployment model this will contain one library. The
console should not assume it, so older/multi-library instances still work.

### `GET /api/system/metrics`

Requires the management token. Example:

```json
{
  "version": "1.2.0",
  "schema_version": 15,
  "uptime_seconds": 86400,
  "libraries": [
    {
      "id": "lib-uuid",
      "name": "Website",
      "assets": 1234,
      "original_bytes": 123456789
    }
  ],
  "storage": {
    "original_bytes": 123456789,
    "preview_bytes": 12345678,
    "database_bytes": 1048576,
    "wal_bytes": 65536
  },
  "jobs": {
    "total": 3,
    "pending": 2,
    "retrying": 1
  },
  "backups": [
    {
      "enabled": true,
      "total": 1234,
      "synced": 1200,
      "pending": 30,
      "failed": 4,
      "last_error": "",
      "last_run_at": "2026-01-02T03:00:00Z"
    }
  ],
  "process": {
    "memory_alloc_bytes": 3612464,
    "memory_sys_bytes": 18073864,
    "goroutines": 5,
    "gc_cycles": 12
  },
  "container": {
    "memory_current_bytes": 73400320,
    "memory_limit_bytes": 536870912,
    "cpu_usage_seconds": 42.5
  }
}
```

`container` fields are best-effort (Linux cgroup v1/v2) and may be absent.

## What the console must do

### Inventory

- Register instances by URL.
- Store library ID/name, environment (prod/staging), region, tags, and an
  optional organization grouping.
- Show the godam version and schema version per instance.
- Detect version/schema drift across the fleet.

### Polling and health

- Poll `/api/health` frequently (e.g. every 15–30s) for liveness.
- Poll `/api/system/info` and `/api/system/metrics` less frequently (e.g. every
  60s), with jitter, backoff, and a per-instance concurrency limit.
- Mark instances offline after N missed heartbeats.
- Never let a slow instance block polling of others.

### Metrics and capacity

- Preserve time-series metrics: storage growth, asset counts, backup success
  and failure, queue depth, memory/CPU, uptime.
- Show current and historical storage usage.
- Estimate disk pressure and growth trends.
- Show per-instance container memory usage vs limit.
- Show whether an instance is at risk of OOM or queue backlog.

### Backup health

- Show enabled/disabled, totals, synced, pending, failed, last success, and last
  error per instance.
- Alert on failures, growing pending counts, and stale last-run timestamps.
- Do not store SFTP credentials or destination secrets.

### Alerts and notifications

Minimum alert set:

- instance unreachable
- backup failed or has not run within the expected window
- pending backup count growing
- storage growth unusually fast
- disk/memory pressure
- version or schema drift
- instance restarted unexpectedly
- certificate expiry (from the edge/proxy layer, if available)

Deliver via email, webhook, Slack, or a generic HTTP endpoint. Include the
instance URL, library name, metric, threshold, and a link to the instance admin
UI.

### Operator links

- Link to each instance’s operator UI.
- Deep-link to its backup settings and status.
- The console itself must not manage assets.

### Token management

- Store each instance’s management token encrypted at rest.
- Support rotation and last-used tracking.
- Offer a “test connection” action that calls `/api/system/info`.
- Keep console tokens separate from library/service tokens used by consuming
  applications.

## What the console must not do

- browse, edit, delete, or deliver assets
- store asset metadata or previews
- store SFTP credentials, passwords, or private keys
- act as a CDN or asset proxy
- become a single point of failure for delivery
- provide a global token that can access every instance’s data plane

## Suggested data model

```text
organizations (optional, display-only)
  id, name

instances
  id, organization_id, url, library_id, library_name,
  environment, region, tags, version, schema_version,
  management_token_encrypted, created_at, last_seen_at, status

instance_metrics
  instance_id, collected_at, uptime_seconds,
  original_bytes, preview_bytes, database_bytes,
  assets, jobs_total, jobs_pending, jobs_retrying,
  backup_synced, backup_pending, backup_failed,
  process_memory_bytes, container_memory_bytes,
  container_memory_limit_bytes, cpu_seconds

alerts
  id, instance_id, type, severity, message,
  first_seen_at, last_seen_at, acknowledged_at, resolved_at
```

Retention should be configurable; aggregate older data instead of storing
per-minute metrics forever.

## Security

- Console and instances communicate over HTTPS.
- One management token per instance; no shared global token.
- Store tokens encrypted; never display them in full after creation.
- Keep `/api/system/*` off the public internet where possible; allow only the
  console IP or a VPN.
- The console is a separate service with its own database and auth.
- The console must be read-only by default.

## Future actions

Possible later, behind a separate elevated scope and explicit confirmation:

- trigger an on-demand backup
- restart an instance
- trigger an upgrade
- collect logs temporarily
- run an integrity check

These should be separate from telemetry and never available to normal
monitoring tokens.

## Failure isolation

- An instance must serve delivery and asset management without the console.
- The console must degrade gracefully when instances are unreachable.
- Polling failures must not affect other instances.
- No instance should block an HTTP request waiting on the console.

## Onboarding checklist

1. Deploy the godam stack (godam + imgproxy) with its own volume and subdomain.
2. Generate a long random `GODAM_MANAGEMENT_TOKEN` and set it on the instance.
3. Verify `/api/health` returns `200`.
4. Verify `/api/system/info` and `/api/system/metrics` work with the token.
5. Register the instance URL and token in the console.
6. Confirm the console can poll and display metrics.
7. Configure alert thresholds and notification channels.
