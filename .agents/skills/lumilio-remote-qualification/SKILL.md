---
name: lumilio-remote-qualification
description: Use when running hardware qualification, scheduling resilience,
  or heavy ADR validation on a remote Docker daemon (e.g. Radxa X4 / Intel N100)
  — execution budget limits, telemetry, disposable QueueDB recovery, and
  destructive resilience testing.
---

# Remote Hardware Qualification & Scheduler Resilience

This skill defines the qualification protocol on dedicated remote hardware
(baseline qualification host: Radxa X4, Intel N100 4C/4T @ 768MB memory limit).

## Audience & Responsibility Boundary

| Skill | Audience | Scope |
| --- | --- | --- |
| `lumilio-e2e-environment` | All contributors | Standard local E2E runs on developer workstations; fast iteration, mock services, zero special hardware requirements. |
| `lumilio-remote-qualification` | Core maintainers & release reviewers | Low-power baseline hardware qualification, real CPU/cgroup memory saturation, and destructive crash-recovery drills. |

### When to Execute Remote Qualification

Run this protocol when landing major architectural changes:
1. Concurrency or scheduling model changes (`execution.Budget`, `DemandCatalog`).
2. Changes to media transcoding or imaging pipelines (ffmpeg thread limits, libvips memory/cache settings).
3. Queue topology and recovery changes (disposable QueueDB contracts, SQLite WAL concurrency).
4. Re-qualifying product throughput baselines before minor/major releases.

## Remote Environment Setup

### 1. Qualification Host Topology (Radxa X4 / Intel N100)
- **CPU**: Intel Processor N100 (4 cores, 4 threads, x86_64).
- **RAM**: 16 GB DDR5 (constrained via container memory budget to 768 MB).
- **OS**: Linux 6.x+ with Docker Engine 24+ and Buildx.

### 2. Passwordless SSH Configuration
Configure the remote host alias in `~/.ssh/config`:

```ssh-config
Host radxa-x4
    HostName 192.168.1.100
    User radxa
    IdentityFile ~/.ssh/id_ed25519
    ControlMaster auto
    ControlPath ~/.ssh/sockets/%r@%h-%p
    ControlPersist 10m
```

Verify connection:
```sh
ssh radxa-x4 docker info
```

### 3. Docker Remote Host Routing
Lumilio's E2E test harness (`web/e2e/support/docker.ts`) transparently routes all Docker and Compose invocations through `LUMILIO_E2E_DOCKER_HOST`:

```sh
export LUMILIO_E2E_DOCKER_HOST="ssh://radxa-x4"
```

### 4. Zero Host-Mount Build Contract
When running against a remote Docker daemon, local host directories cannot be bind-mounted into containers. The E2E stack complies with this:
- Server configuration is baked into the dedicated `e2e` image target in `server/Dockerfile` (`/app/config/server.e2e.toml`).
- Application state and media storage reside exclusively in Docker named volumes (`storage_data`, `app_state_data`).
- Build contexts are transferred over the Docker API streaming protocol.

## Telemetry & Metrics Collection

During a benchmark or qualification run, monitor the container from a secondary terminal:

### 1. CPU Saturation and Concurrency
```sh
docker ${LUMILIO_E2E_DOCKER_HOST:+--host $LUMILIO_E2E_DOCKER_HOST} stats lumilio-photos-e2e-lumilio-1
```

- **Target Peak**: ~375% on 4C/4T (video transcode allocated 3 threads + photo processing allocated 1 thread).
- **Triage**: If CPU stays below 320% during video transcoding, inspect thread allocations in `execution.Budget` or lock contention in libvips.

### 2. Memory Limits and cgroup OOM Detection
Ensure the container stays within the 768 MB budget without tripping the Linux kernel OOM killer:

```sh
docker ${LUMILIO_E2E_DOCKER_HOST:+--host $LUMILIO_E2E_DOCKER_HOST} inspect \
  -f 'OOMKilled={{.State.OOMKilled}}, ExitCode={{.State.ExitCode}}' \
  lumilio-photos-e2e-lumilio-1
```

### 3. SQLite Concurrency & WAL Behavior
Inspect database directory inside the container while under load:

```sh
docker ${LUMILIO_E2E_DOCKER_HOST:+--host $LUMILIO_E2E_DOCKER_HOST} exec \
  lumilio-photos-e2e-lumilio-1 ls -lh /data/app-state/
```

- `catalog.db-wal` should checkpoint steadily and not grow unbounded (> 50 MB indicates write lock starvation).
- `river.sqlite3-wal` should remain lightweight, matching macro job throughput.

## Qualification Test Protocol

### Step 1: Clean Baseline Setup
```sh
LUMILIO_E2E_DOCKER_HOST="ssh://radxa-x4" task web:e2e:up
```

### Step 2: Non-Destructive Benchmark Validation
Run the full media ingestion and video semantic suite against the remote host:

```sh
LUMILIO_E2E_DOCKER_HOST="ssh://radxa-x4" task web:test:video-semantic
```

Verify:
1. End-to-end ingestion throughput matches or exceeds the 1.7 assets/s baseline.
2. All derivative files (thumbnails, transcode profiles, embeddings) complete without error.
3. Health check endpoints (`/api/v1/health/ready`) respond with HTTP 200 within 50ms under load.

### Step 3: Destructive Resilience Testing (QueueDB Loss & Kill -9)
The architectural contract mandates that `catalog.db` holds product truth and `river.sqlite3` is completely disposable.

1. **Inject Hard Crash & Queue Wipe During Ingestion**:
   While a large media ingestion or video transcoding batch is running:
   ```sh
   docker ${LUMILIO_E2E_DOCKER_HOST:+--host $LUMILIO_E2E_DOCKER_HOST} exec \
     lumilio-photos-e2e-lumilio-1 sh -c "rm -f /data/app-state/river.sqlite3* && kill -9 1"
   ```

2. **Restart Container**:
   ```sh
   docker ${LUMILIO_E2E_DOCKER_HOST:+--host $LUMILIO_E2E_DOCKER_HOST} start lumilio-photos-e2e-lumilio-1
   ```

3. **Verify Self-Healing**:
   - Inspect server boot logs:
     ```sh
     docker ${LUMILIO_E2E_DOCKER_HOST:+--host $LUMILIO_E2E_DOCKER_HOST} logs --tail 100 lumilio-photos-e2e-lumilio-1
     ```
   - Verify `river.sqlite3` schema is freshly provisioned automatically.
   - Verify the demand recovery scanner inspects `catalog.db` (`pipeline_stage`, `pipeline_state`, `demand_catalog`).
   - Confirm uncompleted macro tasks resume execution cleanly without duplicate key violations or media file corruption.
   - Check that final state reaches 100% asset readiness.

### Step 4: Teardown
```sh
LUMILIO_E2E_DOCKER_HOST="ssh://radxa-x4" task web:e2e:down
```
