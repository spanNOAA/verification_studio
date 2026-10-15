# HPC-Tools (`hpc-run`) — Read-Only Access V1

**Zero-Install, Application-Level Strict Read-Only Remote Access Tool for NOAA & Partner HPC Clusters**  
Supports: NOAA RDHPCS Gaea (C5/C6/C7 direct gateways), Hera, Ursa, and partner supercomputers (MSU Orion, Hercules).

---

## 🔒 Security Posture & Architecture (HPC Read-Only Access V1)

`hpc-run` operates under a **Strict Minimal Read-Only Security Policy**:

1. **Permanent Removal of Arbitrary Command Execution (P0-1)**:
   - Direct execution of shell command strings (e.g. `hpc-run <cluster> "<cmd>"`) and `--async` are permanently removed.
   - The CLI and background daemon reject any unapproved command with `READ_ONLY_OPERATION_DENIED` (exit code `13`).
2. **Minimal Read-Only API (P0-2)**:
   - Only 3 remote operations are supported: `list`, `stat`, and `download`.
   - All write, upload, delete, rename, permission change (`chmod`/`chown`), directory creation (`mkdir`), and job management (`sbatch`/`srun`) operations are prohibited.
3. **Daemon-Level Default-Deny (P0-3)**:
   - All legal remote commands are generated exclusively by internal rigid read-only templates using `--` option terminators and `shlex.quote()`.
   - Unknown JSON requests or raw command strings sent directly to the socket are rejected immediately by the daemon.
4. **Remote Path Protection (P0-4)**:
   - Rejects PTY control characters (ASCII < 32, linebreaks, DEL).
   - Preserves valid POSIX path characters (spaces, brackets, Unicode, and `$`) safely handled by `shlex.quote()`.
5. **Local Socket Security & Protocol Isolation (P1-5)**:
   - Daemon listens on dedicated read-only socket namespace: `~/.ssh/hpc_ro_{cluster}.sock`.
   - Base directory restricted to `0700` and socket file restricted to `0600`.
   - Socket connections verify peer process UID.
   - Legacy daemons on `hpc_{cluster}.sock` are safely detected and prompted for migration without blind `SIGKILL`.
6. **Download Safety & Atomic Local Write (P1-7)**:
   - Single-file download limit enforced at 50 MB (remote size pre-check and socket receive limits).
   - Local downloads use directory temporary files and atomic replacement (`os.replace`), rejecting symlink overwrite attacks.

---

## 📋 CLI Usage & Permitted Operations

### Connection Management
```bash
hpc-run --start gaea-c6     # Interactive PIN + YubiKey authentication
hpc-run --status [cluster]  # Check daemon status
hpc-run --stop [cluster]    # Stop read-only background daemon
```

### Remote Read-Only Operations
```bash
# 1. Browse directory entries
hpc-run gaea-c6 list /gpfs/f6/project/

# 2. Query file metadata (size, mtime, permissions)
hpc-run gaea-c6 stat /gpfs/f6/project/image.png

# 3. Download remote file safely to local path (Max 50 MB)
hpc-run gaea-c6 download /gpfs/f6/project/image.png ~/Desktop/image.png
```

### Prohibited Operations (Default-Deny)
```bash
hpc-run gaea-c6 "rm -rf something"
# [SECURITY BLOCKED] READ_ONLY_OPERATION_DENIED (Exit code 13)

hpc-run gaea-c6 --async "python run.py"
# [SECURITY BLOCKED] READ_ONLY_OPERATION_DENIED (Exit code 13)

hpc-run gaea-c6 upload local.png remote.png
# [SECURITY BLOCKED] READ_ONLY_OPERATION_DENIED (Exit code 13)
```

---

## 🧪 Automated Security Test Suite

Run the built-in test suite to verify all read-only constraints, path sanitization, and socket protocol defenses:

```bash
python3 test_hpc_readonly.py
```
