# CHANGELOG.md

## Rictus Changelog

# [0.3.2-stable] — 2025-10-17
** Overview **
Initial stable milestone for the Rictus Core Daemon.

** Added **
 - Core controller, internal queue, and agent execution engine.
 - Built-in HTTP API: /healthz, /version, /agents, /tasks, /ingest, /reload.
 - Hot-reload watcher (enabled by default).
 - Digit agent support for outbound analysis and data collection.
 - STN API integration — only Rictus pushes data upstream.
 - Local logging under ./data/logs/ (no system paths).
 - Multi-arch build targets and Pi4 deploy helper.
 - Systemd service template for optional autostart.

** Notes **
 - Agents may communicate with Rictus, but only Rictus may push to STN.
 - Config schema version locked at rictus.json v0.3.2.
 - All runtime files reside in ./data; no root-level dependencies.

** Next **
 - Implement automatic key rotation for STN ingest.
 - Add lightweight metrics endpoint.
 - Expand watcher to include file-hash verification.

README.md

Rictus Core v0.3.2-stable

Rictus is the control daemon for the STN-Labz ecosystem.
It coordinates internal agents, performs data collection, and serves as the only authorized bridge to the STN API.

Overview

Agents talk to Rictus.

Only Rictus pushes to the STN API.

Runs locally, self-contained under ./data/.

Lightweight and cross-platform (ARM64, AMD64, ARMv7, Darwin).

Configuration
Default configuration file: data/rictus.json
(node_id, data_dir, stn_api, stn_ingest_secret, digit, auto, max_workers, queue_capacity)

API Endpoints
GET /healthz – Check if daemon is alive
GET /version – Return build version info
GET /agents – List active agents
POST /tasks – Enqueue a new task
POST /ingest – Rictus-only STN ingest push
POST /reload – Reload configuration

Build Targets
make build
make build-linux-arm64
make build-linux-amd64
make install-arm64 HOST=pi@<ip>

License and Ownership
Copyright © 2025 STN-Labz
Internal Project — Top Secret / Confidential
Rictus Core and its derivatives are property of STN-Labz and must not be distributed, mirrored, or shared outside of authorized infrastructure.

Version
v0.3.2-stable
