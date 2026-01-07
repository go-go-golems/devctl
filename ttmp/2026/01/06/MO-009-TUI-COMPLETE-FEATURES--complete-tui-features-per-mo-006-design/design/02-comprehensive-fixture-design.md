---
Title: Comprehensive TUI Fixture Design
Ticket: MO-009-TUI-COMPLETE-FEATURES
Status: active
Topics:
    - testing
    - fixtures
DocType: design
Intent: long-term
Owners: []
RelatedFiles:
    - scripts/setup-comprehensive-fixture.sh
ExternalSources: []
Summary: "Design for a comprehensive fixture repo that exercises all TUI features"
LastUpdated: 2026-01-07T04:30:00-05:00
WhatFor: "Document what TUI features need testing and how the fixture exercises them"
WhenToUse: "When setting up test environments or extending fixture coverage"
---

# Comprehensive TUI Fixture Design

## Overview

This document describes the design of a comprehensive fixture repository for testing the `devctl tui`. Unlike the basic MO-006 fixture, this fixture exercises **all** TUI features built in MO-009, including:

- Multiple services with varying behaviors
- Health checks (TCP and HTTP)
- Process statistics display
- Environment variable display
- Log levels and filtering
- Pipeline with progress and live output
- Config patches from plugins
- Multiple plugins with different capabilities

---

## Feature Coverage Matrix

| Feature | TUI Location | How Fixture Exercises It |
|---------|--------------|--------------------------|
| Service listing | Dashboard | Multiple services (4-5) |
| Health status | Dashboard + Service Detail | TCP and HTTP health checks |
| CPU/Memory stats | Dashboard + Service Detail | Long-running services |
| Service uptime | Service Detail | Started timestamps |
| Recent events | Dashboard | High-frequency log producers |
| Log levels | Events View | Services producing DEBUG/INFO/WARN/ERROR |
| Service filtering | Events View | Multiple services to filter |
| Level filtering | Events View | Mixed log levels |
| Event rate stats | Events View | High-throughput service |
| Process info | Service Detail | PIDs, commands, cwd |
| Environment vars | Service Detail | Env vars passed to services |
| Build progress | Pipeline View | Simulated build with progress |
| Live output | Pipeline View | Build producing output |
| Config patches | Pipeline View | Plugin emitting patches |
| Plugin list | Plugins View | Multiple plugins configured |
| Plugin details | Plugins View | Different plugin priorities |

---

## Fixture Services

### 1. `backend` - HTTP Server with Health Check

**Purpose**: Test HTTP health checks, CPU usage display, log output

**Behavior**:
- HTTP server listening on dynamic port
- Health endpoint at `/health`
- Responds to requests with JSON
- Logs requests at INFO level
- Moderate CPU usage when handling requests

**Environment**:
```
PORT=<dynamic>
ENV=development
DB_URL=postgresql://localhost:5432/testdb
```

### 2. `worker` - Background Job Processor

**Purpose**: Test TCP health checks, high CPU usage display

**Behavior**:
- Listens on TCP port (for health check)
- Simulates CPU-intensive work
- Logs at DEBUG and INFO levels
- Runs continuously

**Environment**:
```
WORKER_CONCURRENCY=4
REDIS_URL=redis://localhost:6379
API_KEY=********  (should be redacted)
```

### 3. `log-spewer` - Multi-Level Log Producer

**Purpose**: Test log level filtering, event rate, event volume

**Behavior**:
- Produces logs at all levels (DEBUG, INFO, WARN, ERROR)
- Configurable rate (e.g., 10-50 logs/sec)
- Random log messages to simulate realistic output
- No health check (tests "unknown" health state)

**Environment**:
```
LOG_RATE=20
LOG_DURATION=300
```

### 4. `database` - Simulated Database

**Purpose**: Test service with exit, stderr output

**Behavior**:
- Starts and runs for a while
- Produces some INFO logs
- Eventually exits (simulating crash or intentional shutdown)
- Writes to stderr on exit

**Environment**:
```
DATA_DIR=/tmp/fixture-db
MAX_CONNECTIONS=100
```

### 5. `flaky-service` - Intermittent Failures

**Purpose**: Test unhealthy status, service restart scenarios

**Behavior**:
- HTTP health check that sometimes fails
- Alternates between healthy and unhealthy
- Logs warnings when unhealthy
- Tests the ERROR log level display

**Environment**:
```
FAILURE_RATE=0.3
```

---

## Plugin Configuration

### Plugin 1: `config-mutator` (Priority 10)

**Purpose**: Test config patches display, plugin list view

**Capabilities**:
- Op: `mutate_config`
- Emits config patches during pipeline

**Patches Emitted**:
```
services.backend.port → 8083
services.worker.concurrency → 4
build.cache_enabled → true
```

### Plugin 2: `build-runner` (Priority 20)

**Purpose**: Test build step progress, live output

**Capabilities**:
- Op: `build`
- Stream: `build.output`

**Behavior**:
- Emits build progress (0% → 100% over 10-15 seconds)
- Produces live output lines
- Simulates multi-step build process

### Plugin 3: `validator` (Priority 30)

**Purpose**: Test validation errors/warnings display

**Capabilities**:
- Op: `validate`

**Behavior**:
- Emits 1-2 warnings (non-blocking)
- Tests validation result display in pipeline view

---

## Build/Pipeline Simulation

The fixture includes a plugin that simulates a realistic build process:

### Build Phases

1. **Config Mutation** (2s)
   - Emits 3 config patches

2. **Build** (10-15s)
   - Step 1: "Compiling dependencies" (0% → 30%)
   - Step 2: "Building backend" (30% → 60%)
   - Step 3: "Building worker" (60% → 90%)
   - Step 4: "Linking" (90% → 100%)
   - Each step emits live output

3. **Prepare** (3s)
   - Step 1: "Creating directories"
   - Step 2: "Copying assets"

4. **Validate** (1s)
   - Warning: "Deprecated config key 'legacy_mode'"
   - Warning: "Port 8080 commonly conflicts with other services"

5. **Launch Plan**
   - Lists all 5 services

6. **Supervise**
   - Starts services

---

## Test Scenarios

### Scenario 1: Fresh Start

1. Run fixture setup script
2. Start `devctl tui`
3. Observe dashboard populating with services
4. Check health icons appear (initially unknown, then healthy/unhealthy)
5. Verify CPU/MEM columns show values

### Scenario 2: Service Detail Inspection

1. Select a service from dashboard
2. Press Enter to view logs
3. Verify:
   - Process info box shows PID, command, cwd, CPU, MEM, uptime
   - Health box shows status and endpoint
   - Environment box shows sanitized vars
   - Log viewport scrolls and follows

### Scenario 3: Event Filtering

1. Switch to Events view (Tab)
2. Observe events from multiple services
3. Toggle service filters (number keys)
4. Toggle level filters
5. Verify event rate display
6. Test pause (p key)

### Scenario 4: Pipeline View

1. Run `devctl up` to trigger pipeline
2. Switch to Pipeline view
3. Observe:
   - Phase progress indicators
   - Build step progress bars
   - Live output viewport
   - Config patches section
   - Validation warnings

### Scenario 5: Plugin Inspection

1. Switch to Plugins view (Tab through views)
2. Observe 3 plugins listed
3. Expand each plugin
4. Verify capabilities shown correctly

### Scenario 6: Service Failure

1. Wait for `database` service to exit
2. Observe:
   - Dashboard shows service as dead
   - Exit info appears in service detail
   - Error event appears in events view

### Scenario 7: Terminal Resize

1. Resize terminal during operation
2. Verify all views adapt correctly
3. No rendering artifacts

---

## Implementation Notes

### Script Requirements

The setup script should:

1. Create temp directory (`mktemp -d`)
2. Build test binaries from `testapps/`:
   - `http-echo` (already exists)
   - `log-spewer` (already exists)
   - May need new: `tcp-echo`, `flaky-server`
3. Find free ports for each service
4. Generate `.devctl.yaml` with all plugins
5. Create plugin scripts in fixture
6. Print REPO_ROOT path

### Test App Extensions

May need to extend existing test apps or create new ones:

**`log-spewer` enhancements**:
- Add `--levels` flag to control which levels to produce
- Add `--rate` flag for configurable output rate

**New `flaky-server`**:
- HTTP server with configurable failure rate
- Health endpoint that sometimes returns 500

**New `tcp-echo`**:
- Simple TCP server for health check testing
- Echoes back any input

### Plugin Scripts

Each plugin can be a simple Python or bash script that:
- Responds to `introspect` with capabilities
- Handles specific ops
- Emits appropriate events via stdout JSON

---

## File Structure

```
$REPO_ROOT/
├── .devctl.yaml                 # Main config with all plugins
├── bin/
│   ├── http-echo               # HTTP server binary
│   ├── log-spewer              # Log producer binary
│   ├── tcp-echo                # TCP server binary
│   └── flaky-server            # Flaky HTTP server binary
└── plugins/
    ├── config-mutator.py       # Config mutation plugin
    ├── build-runner.py         # Build simulation plugin
    └── validator.py            # Validation plugin
```

---

## Success Criteria

The fixture is successful if:

1. All 5 services start and produce visible output
2. Health checks show appropriate states (healthy/unhealthy/unknown)
3. CPU/MEM stats update every few seconds
4. Events view shows all log levels from multiple sources
5. Pipeline view shows progress during `devctl up`
6. Config patches appear in pipeline view
7. All 3 plugins appear in plugins view with correct details
8. At least one service exits to test exit info display
9. No visual artifacts or layout issues in any view
10. Tab cycling through all views works correctly

