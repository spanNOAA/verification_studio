# HPC-Tools (hpc-run)

**Zero-Install, Portable, Persistent Multi-Cluster SSH Execution Suite**  
Designed for NOAA RDHPCS (Gaea C5/C6/C7, Hera, Ursa) and partner supercomputers (MSU Orion, Hercules).

---

## Core Design Principles

1. **Zero-Install & Zero-Dependencies**: Built entirely using the Python 3 standard library -- **no third-party dependencies** (no `pip install` required). Clone or copy the folder and run immediately.
2. **Zero Code Modification**:
   - **Adaptive Username & Shell Matching**: Universal regex patterns automatically match any user's shell prompt across all clusters, supporting Bash, Csh, Tcsh, and Zsh without editing source code.
   - **Intelligent SSH Config Detection**: Automatically detects host aliases defined in `~/.ssh/config` (such as `Gaeac5`, `gaea-c5`, `c5`, `Hera`, `ursa`, etc.). Even if `~/.ssh/config` is completely empty, it automatically falls back to canonical FQDNs and injects OpenSSH ControlMaster multiplexing.
   - **Automatic Socket Directory Management**: Automatically verifies and creates `~/.ssh/sockets` with secure permissions (`0700`), preventing connection failures.
   - **Pure Relative Symlinks**: All cluster alias symlinks (`gaea-run -> hpc-run`, `c5-run -> hpc-run`, etc.) use relative paths, ensuring portability across any installation directory.
   - **Cross-Platform**: Supports macOS and Linux.

---

## Included Tools

| Command | Alias / Shortcut | Target Cluster / Description |
| :--- | :--- | :--- |
| `hpc-run` | `hpc-run <cluster> "<cmd>"` | Universal execution dispatcher (supports all clusters) |
| `gaea-run` | `gaea-run "<cmd>"` | NOAA RDHPCS Gaea (auto-routes to online or default partition) |
| `gaea-c5-run` | `c5-run "<cmd>"` | Gaea C5 direct session (login nodes `gaea51`~`56`, `/gpfs/f5`) |
| `gaea-c6-run` | `c6-run "<cmd>"` | Gaea C6 direct session (login nodes `gaea61`~`66`, `/gpfs/f6`) |
| `gaea-c7-run` | `c7-run "<cmd>"` | Gaea C7 direct session (login nodes `gaea71`~`76`, `/gpfs/f7`) |
| `hera-run` | `hera-run "<cmd>"` | NOAA RDHPCS Hera |
| `ursa-run` | `ursa-run "<cmd>"` | NOAA RDHPCS Ursa |
| `orion-run` | `orion-run "<cmd>"` | MSU Orion |
| `hercules-run`| `hercules-run "<cmd>"` | MSU Hercules |

---

## Quick Start

### 1. Ready to Run
Add the directory to your `PATH`, or execute directly using relative paths:
```bash
# Option A: Add to PATH (recommended)
export PATH="$HOME/.local/bin:$PATH"  # or your cloned directory path

# Option B: Run directly
./hpc-run --status
```

### 2. Check Cluster Status
```bash
hpc-run --status          # View status of all clusters
hpc-run --status gaea     # View Gaea partition status
```

### 3. Session Initialization & Keepalive
Because NOAA and partner HPC gateways enforce multi-factor authentication (MFA / YubiKey), run the interactive starter once per session:
```bash
# Start a persistent daemon session (enter PIN + touch YubiKey once)
hpc-run --start gaea-c6
hpc-run --start hera
hpc-run --start ursa
```
> **Tip**: With OpenSSH `ControlPersist` configured, sessions remain active headlessly for up to 72 hours, delivering sub-0.1s instant command execution.

### 4. Command Execution (0.1s Instant Response)
```bash
# Gaea direct partition execution:
gaea-c5-run "df -h /gpfs/f5"
gaea-c6-run "squeue -u $USER"
gaea-c7-run "sinfo -p batch"
gaea-run "hostname"                   # Auto-routes to currently active Gaea session
gaea-run --node gaea62 "hostname"     # Targets specific login node

# Other clusters:
hera-run "squeue -u $USER"
ursa-run "hostname"
orion-run "qstat"
hercules-run "sinfo"

# Universal syntax:
hpc-run hera "hostname"
hpc-run c6 "df -h"
```

### 5. Asynchronous Background Execution (`--async`)
For long-running tasks where waiting is not desired, add `--async`:
```bash
gaea-c6-run --async "python run_model.py > output.log 2>&1"
# Returns remote PID and log location immediately without blocking
```

### 6. Interrupt & Session Management
```bash
# Abort a hanging remote command (forwards SIGINT to remote PTY)
hpc-run --cancel gaea-c6

# Stop persistent session daemon
hpc-run --stop gaea-c6
hpc-run --stop all
```

---

## Optional Environment Variables

The tool relies entirely on automatic discovery and default conventions. No manual configuration is required. However, you may customize behavior via environment variables if desired:

| Variable | Default | Description |
| :--- | :--- | :--- |
| `HPC_GAEA_DEFAULT` | Auto-detected active session, fallback `gaea-c5` | Default preferred partition for generic `gaea-run` (`c5`, `c6`, `c7`) |
| `HPC_USER` | System `$USER` or SSH config user | Overrides remote username for all clusters |
| `HPC_<CLUSTER>_USER` | Same as above | Overrides username for a specific cluster (e.g. `HPC_ORION_USER=span`) |

---

## AI Agent & Automation Integration

In AI programming assistant environments (Antigravity, Gemini, Claude Code, Cursor):
- **NEVER execute raw interactive `ssh` commands** (MFA prompts and load-balancer banners cause non-interactive agent sessions to hang or time out).
- **Always invoke remote commands via `hpc-run <cluster> "<cmd>"` or alias runners**.
- If a cluster is `OFFLINE`, prompt the user to run `hpc-run --start <cluster>` in an interactive terminal once.
