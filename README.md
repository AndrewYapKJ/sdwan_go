# go-sdwan — Quick Start, Configuration & Improvement Guide

This repository contains a minimal production-ready SD‑WAN prototype (WireGuard-based): a single `hub` (aggregator) and multiple `cpe` (branch) binaries with zero-config enrollment, a small dashboard and Prometheus metrics.

This README contains three short guides:
- **Installation & Setup** — build and install on macOS and Debian
- **Configuration Settings** — flags, environment variables and systemd examples
- **Improvement Guide** — recommended items to harden and extend the project

---

**Installation & Setup**

- Prerequisites (build machine):
  - Go 1.23+ installed on macOS (Homebrew: `brew install go`).
  - `make` (comes with Xcode Command Line Tools). 

- Build static Linux binaries (on macOS):
  ```bash
  cd /Volumes/REPO/PERSONAL/sdwan_go
  go mod tidy
  make all
  # artifacts -> bin/hub-linux-amd64, bin/hub-linux-arm64, bin/cpe-linux-*
  ```

- Quick control‑plane test on a single mac (no WireGuard/kernel privileges)
  1. Build native mac binaries (optional):
     ```bash
     go build -o bin/hub-macos ./hub
     go build -o bin/cpe-macos ./cpe
     ```
 2. Start hub without configuring `wg0` (no root required):
     ```bash
     ./bin/hub-macos --no-wg --listen 127.0.0.1:8080
     ```
 3. Generate token (dashboard or):
     ```bash
     curl -s -X POST http://127.0.0.1:8080/api/token
     ```
 4. Register a CPE (no WireGuard):
     ```bash
     ./bin/cpe-macos --hub http://127.0.0.1:8080 --token <TOKEN> --no-wg
     ```
 5. Check the dashboard: `http://127.0.0.1:8080/dashboard`

- Full dataplane test (recommended): run two Linux VMs (Multipass/VMs)
  - Build Linux static binaries (`make all`), transfer to the VMs, install `wireguard` and run hub + cpe (see *Testing* section below).

- One‑click Debian installer (deploy on target machines)
  - The script `scripts/install-debian.sh` installs WireGuard, downloads release binaries and sets up a systemd service for hub or cpe. Example on hub:
    ```bash
    wget https://raw.githubusercontent.com/yourname/go-sdwan/main/scripts/install-debian.sh
    chmod +x install-debian.sh
    sudo ./install-debian.sh YOUR_HUB_PUBLIC_IP
    # choose 'h' when prompted
    ```

**Testing**

- Quick control‑plane (macOS): use `--no-wg` as shown above.
- Full WireGuard testing (two VMs):
  1. Create two VMs (Multipass example):
     ```bash
     multipass launch --name hub --mem 2G --disk 10G
     multipass launch --name cpe1 --mem 1G --disk 5G
     ```
 2. Transfer and install binaries, install `wireguard` and run services (see earlier message for exact commands).
 3. Inspect interface: `sudo wg` and `ip addr show wg0` to verify tunnels.

**Configuration Settings**

- Hub flags & envs (defaults shown)
  - `--listen` or env `LISTEN` (default `0.0.0.0:8080`) — HTTP dashboard/API listen address
  - env `WG_PORT` (default `51820`) — WireGuard listen UDP port
  - `--no-wg` — skip wireguard device configuration (testing only)

- CPE flags
  - `--hub` — hub base URL (e.g. `https://hub.example.com:8080`)
  - `--token` — enrollment token obtained from hub
  - `--no-wg` — skip wg device configuration (useful for testing registration)

- Systemd examples (installer creates these; you may tune them):
  - Hub unit (`/etc/systemd/system/sdwan-hub.service`):
    ```ini
    [Unit]
    Description=Go SD-WAN Hub
    After=network.target

    [Service]
    ExecStart=/usr/local/bin/sdwan-hub --listen 0.0.0.0:8080 --wg-port 51820
    Restart=always
    RestartSec=5

    [Install]
    WantedBy=multi-user.target
    ```
  - CPE unit (`/etc/systemd/system/sdwan-cpe.service`):
    ```ini
    [Unit]
    Description=Go SD-WAN CPE
    After=network.target

    [Service]
    ExecStart=/usr/local/bin/sdwan-cpe --hub https://HUB:8080 --token <TOKEN>
    Restart=always
    RestartSec=5

    [Install]
    WantedBy=multi-user.target
    ```

**Where files and state live today**
- The hub stores keys, tokens and peer allocations in-memory only. There is no persistent storage by default; reboots will forget state.
- WireGuard device used: `wg0` (kernel-managed on Linux via `wgctrl`).

**Troubleshooting**
- If `go` is missing: `brew install go` and ensure `/opt/homebrew/bin` or `/usr/local/bin` is on your `PATH`.
- If `go mod tidy` fails for `wgctrl`: run `go get golang.zx2c4.com/wireguard/wgctrl@latest` then `go mod tidy`.
- If service fails to create `wg0`: run hub/cpe with `--no-wg` to verify control-plane first; on Linux ensure kernel module `wireguard` is installed or `wireguard-go` userspace is available.

---

**Improvement Guide & Roadmap (recommended priority order)**

1. Persistence & Hot-Restart (High)
   - Store hub private key, tokens and peer assignments to disk (simple JSON file). Load them on startup so reboots don't require re-enrollment.

2. Secure Transport (High)
   - Serve hub API over TLS (builtin or behind reverse proxy like nginx/traefik). Add optional mutual TLS for CPEs.

3. Endpoint Discovery & NAT Traversal (High)
   - Return peer endpoints to CPEs during registration so peers can attempt direct connections.
   - Implement hub-mediated UDP hole-punching or a relay fallback (TURN‑style) for symmetric NATs.

4. Persistent AllowedIPs + Routing (Medium)
   - Generate full WireGuard configs with `AllowedIPs` for routing subnets through the hub or between spokes.

5. Production Hardening (Medium)
   - Run hub as dedicated non-root user where possible and tighten `systemd` security options.
   - Add logging/structured logs, rotate logs, and expose debug levels.

6. Packaging & Releases (Medium)
   - Add `goreleaser` config to publish Linux static binaries and GitHub releases automatically.

7. Monitoring & Observability (Low)
   - Expand Prometheus metrics (peer counts, re-enrollment, errors) and provide a Grafana dashboard example.

8. Features & Extensions (Optional)
   - Multi‑WAN failover on CPE
   - Application-aware routing / BGP integration
   - Docker / containerized deployment

---

If you want, I can implement the top items incrementally: (1) persistence, (2) TLS + reverse proxy example, then (3) endpoint propagation & relay. Tell me which to start and I will update the code and tests accordingly.

Enjoy — feel free to ask for a `multipass` script to run the two-VM dataplane test automatically.
