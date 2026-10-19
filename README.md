# NOAA GSL Verification Studio

[![GitHub Release](https://img.shields.io/github/v/release/spanNOAA/verification_studio?color=blue&label=Latest%20Release)](https://github.com/spanNOAA/verification_studio/releases/latest)
[![macOS Universal](https://img.shields.io/badge/Platform-macOS%20(Apple%20Silicon%20%2B%20Intel)%20%7C%20Linux%20%7C%20Windows-brightgreen.svg)](https://github.com/spanNOAA/verification_studio/releases/latest)
[![Binary Size](https://img.shields.io/badge/Binary%20Size-13%20MB-blue.svg)](https://github.com/spanNOAA/verification_studio/releases/latest)
[![DMG Size](https://img.shields.io/badge/DMG%20Installer-5.8%20MB-success.svg)](https://github.com/spanNOAA/verification_studio/releases/latest)
[![License](https://img.shields.io/badge/License-NOAA%20GSL-informational.svg)](https://github.com/spanNOAA/verification_studio)

An end-to-end automated verification and presentation generation suite engineered for numerical weather prediction models (e.g. RRFS, HRRR). By specifying experiment model identifiers and verification date ranges, Verification Studio **automatically evaluates NOAA GSL MATS statistical curves, retrieves remote HPC radar reflectivity graphics**, and **generates professional 26-slide PowerPoint presentations (.pptx) and 1:1 synchronized interactive web presentations (.html) with a single click**.

> [!IMPORTANT]
> **macOS First-Time Launch Notice (Gatekeeper Workaround)**
> Verification Studio is an open-source scientific tool built in pure Go and is distributed without an Apple commercial Developer ID ($99/year). If macOS displays *"Apple could not verify this app is free of malware..."* or blocks the initial launch:
>
> - **Option 1 (System Settings - Recommended for macOS 15 Sequoia / Sonoma)**:
>   1. Attempt to launch `Verification Studio.app` once so macOS registers the attempt.
>   2. Open **System Settings** -> **Privacy & Security**.
>   3. Scroll down to the **Security** section and click **"Open Anyway"** (or **"Allow Anyway"**).
>   4. Authenticate with your Mac password or Touch ID. *(Required only on the very first launch)*.
>
> - **Option 2 (Terminal - Instant One-liner)**:
>   Run the following command in Terminal to clear the download quarantine attribute:
>   ```bash
>   xattr -cr "/Applications/Verification Studio.app"
>   ```

---

## 1. Modern Architecture (Pure Go Universal Engine)

Verification Studio v2.0 has been re-architected from the ground up into a **100% pure Go standalone application**:

1. **Zero Python / Node.js Dependencies**:
   - The entire verification engine, HTTP automation, Plotly image extraction, PowerPoint compiler, and user interface are written in Go.
   - No Python runtime, `pip`, virtual environments, or heavy third-party packages required.
2. **Zero Bundled Browsers**:
   - Directly reuses host system browsers (**Google Chrome**, **Microsoft Edge**, **Brave**, **Chromium**, or **Arc**) via native Chrome DevTools Protocol (CDP).
   - Eliminates 400MB+ bundled headless browser downloads.
3. **Ultra-Lightweight Footprint**:
   - Standalone application binary reduced from 471 MB to **13 MB**.
   - Universal macOS DMG installer reduced from 222 MB to **5.8 MB**.
4. **Intelligent Cloudflare & MATS Rate-Limit Auto-Heal**:
   - Implements request pacing and exponential backoff with jitter to stay well within API thresholds.
   - Automatically detects server-side rate limits (HTTP 429, 503, 504, or navigation deadline exceeded) and triggers an automated **3-minute cooldown timer**.
   - Gracefully re-initializes a fresh browser tab and session, resuming verification without human intervention.
5. **Per-Stage Browser Session Strong Isolation**:
   - Each of the 10 verification stages (Surface Dieoff, Surface Diurnal TS, Upper Air Dieoff, Profiles, Aircraft AMDAR, Radar Reflectivity, Cloud Ceiling, Visibility, HPC Radar, and MRMS Observations) runs in an isolated browser session.
   - Eliminates cross-domain state pollution, stale DOM contexts, and stuck CDP targets.
6. **Headless HPC Session Auto-Connect**:
   - The built-in read-only HPC engine automatically leverages active SSH `ControlMaster` connections when available.
   - Reconnects in 1 to 2 seconds in the background without launching terminal popups or requiring repeated YubiKey touches.
   - Interactive PIN + YubiKey terminal fallback is launched only when active credentials have expired.
7. **Universal Multi-Cluster Shell Prompt Detection**:
   - Robust multi-dimensional prompt detection engine supporting all NOAA RDHPCS clusters (Gaea C5/C6/C7, Hera, Ursa) and partner supercomputers (MSU Orion, Hercules).
   - Automatically handles non-standard login hostnames (e.g. `c6login1`, `c7login1`), ANSI color escape sequences, and modern terminal Bracketed Paste Mode (`\x1b[?2004h`).
8. **Force Fetch & Safe Cache Control**:
   - Features an optional Force Fetch toggle with a safety confirmation modal to cleanly bypass local plot caches when fresh evaluation runs are desired.
   - Failed plot slots are gracefully logged and left cleanly formatted in slides without aborting the pipeline.
9. **Enhanced HD Typography & Dynamic Chart Layout**:
   - Dynamic canvas styling injects readable font sizes into Plotly graphs (Title 26px, Axes 20px, Ticks/Legend 16px).
   - Charts remain crystal-clear, bold, and comfortably readable when scaled into 3-panel slides.
10. **Multi-Domain Regional Evaluation**:
    - Seamlessly evaluates Full Domain (`all`), Eastern Region (`east`), and Western Complex Terrain (`west`) verification decks.
    - Interactive HTML presentation features instant region switching tabs.
11. **Native Desktop Experience**:
    - Modern native user interface with embedded real-time progress bars, live log console, and one-click folder browsing.

---

## 2. Comprehensive Verification Coverage

| Category | Verification Source | Evaluated Variables & Metrics |
| :--- | :--- | :--- |
| **Surface Dieoff** | METAR station observations (Plot Matched) | 2-m Temperature, 2-m Dewpoint, 10-m Wind Speed (RMSE & BIAS) |
| **Surface Diurnal Time Series** | METAR station observations | 1-h, 3-h, 12-h, and 24-h forecast diurnal cycles across metrics |
| **Upper Air Dieoff** | Traditional Rawinsonde (RAOBs) soundings | Temperature, Wind Speed, Relative Humidity (RHobT) |
| **Upper Air Profiles** | RAOBs soundings (1000 hPa to 100 hPa) | 1-h, 3-h, 6-h, and 12-h vertical profile soundings |
| **Aircraft Observations** | AMDAR traditional flights | Temperature, Wind Speed, and Humidity aloft |
| **Cloud Ceiling** | METAR aviation thresholds | Dieoff & Diurnal Time Series at < 500 ft, < 1000 ft, < 3000 ft (CSI & BIAS) |
| **Visibility** | METAR aviation thresholds | Dieoff & Diurnal Time Series at < 1 mi, < 3 mi, < 5 mi (CSI & BIAS) |
| **Radar Reflectivity** | NSSL MRMS Quantitative Radar | 15 dBZ, 25 dBZ, 35 dBZ Dieoff & Diurnal Time Series |
| **Synoptic Radar Mosaic** | Remote HPC model output + MRMS Obs | Side-by-side multi-model synoptic case comparisons |

---

## 3. Usage Guide (Native Desktop App)

1. **Download & Install**:
   - Download [`Verification_Studio_macOS_Universal.dmg`](https://github.com/spanNOAA/verification_studio/releases/latest) for macOS, or download the Linux / Windows release package.
   - On macOS, open the DMG and drag **`Verification Studio.app`** to your `/Applications` folder.
2. **Launch Application**:
   - Double-click **Verification Studio** from Launchpad, Finder, or `/Applications`.
   - *Note for macOS users*: If Gatekeeper displays an unverified app warning on first launch, approve it via **System Settings -> Privacy & Security -> Open Anyway** (see notice above).
3. **Configure Settings**:
   - **Report Title**: e.g., `RRFSv2 Model Verification & Performance Evaluation`
   - **Output Directory**: Click `Browse...` to pick a destination folder on your Desktop or workspace.
   - **Models**: Enter or paste experiments (one per line):
     ```text
     retv4:RRFSv2_conus_3km_ret_v2x_retv4 - gaeac6:/gpfs/f6/bil-fire10-oar/world-shared/jjh/rrfsv2/RRFSv2X/L60/OPSROOT/RRFSv2X.retv4
     generictquv:RRFSv2_conus_3km_ret_generictquv
     halfTerr:RRFSv2_conus_3km_ret_halfTerr - gaeac6:/gpfs/f6/wrfruc/scratch/username/retros/RRFSv2X.halfTerr
     ```
     *Format: `Experiment_ID - machine:path` or simply `Experiment_ID`. Supported clusters include `gaeac5`, `gaeac6`, `gaeac7`, `hera`, `ursa`, `orion`, and `hercules`. Model aliases (e.g. `retv4`, `halfTerr`) are inferred automatically.*
   - **Date Ranges**: Specify Surface (METAR) and Upper-Air (RAOBs/AMDAR) date ranges.
   - **Evaluated Regions**: Toggle Full Domain (`all`), Eastern (`east`), and Western Complex Terrain (`west`).
   - **Force Fetch (Optional)**: Check to force re-downloading all figures, bypassing local disk caches.
4. **Generate**:
   - Click **`Start Automated Generation`** and monitor the live dual-tier progress bar and output logs.
   - Upon completion, synchronized `.pptx` and `.html` presentation decks are instantly available in your destination directory.

---

## 4. Hardened Native Pure Go HPC Integration (RO-v1 Engine)

Verification Studio includes a **built-in, native Pure Go HPC read-only engine** for secure, self-contained remote access to NOAA supercomputers with zero external dependencies:

- **Supported Supercomputers**: NOAA RDHPCS Gaea (`gaea-c5`, `gaea-c6`, `gaea-c7`), Hera (`hera`), Ursa (`ursa`), and university partners (MSU `orion`, `hercules`).
- **Zero Python / Zero Dependencies**: Fully implemented in pure Go. Remote radar retrieval works out-of-the-box on distributed systems without requiring Python, helper scripts, or package installations.
- **Strict Read-Only Security Compliance (RO-v1)**:
  - Strict capability whitelist: only `list`, `stat`, and `download` operations are structurally permitted.
  - Arbitrary remote shell command execution, file writing, modification, deletion, and batch job submissions (`sbatch`, `qsub`) are permanently blocked by the protocol daemon.
  - Local Unix domain sockets operate under strict `0600` permissions with peer process UID verification.
  - Safe atomic file replacement with symlink traversal protection and 50 MB download payload limits.
- **Zero-Config Fallback & Auto-Teardown**:
  - Automatically resolves hostnames to official Canonical FQDNs if `~/.ssh/config` is unconfigured.
  - Background HPC session tunnels automatically disconnect and clean up socket files upon verification report completion, preventing lingering background processes.
- **Built-in CLI Commands**:
  ```bash
  # Check cluster connection status
  ./verification_studio hpc-run --status

  # Authenticate PIN + YubiKey interactively (if offline)
  ./verification_studio hpc-auth gaea-c6

  # Standalone read-only operations
  ./verification_studio hpc-run gaea-c6 list /gpfs/f6/scratch1
  ./verification_studio hpc-run gaea-c6 download /path/to/cref.png ./local.png

  # Stop background session daemons
  ./verification_studio hpc-run --stop all
  ```

---

## 5. Multi-Platform Releases & Distribution

Verification Studio packages and releases cross-platform standalone binaries automatically via GitHub Actions:

| Platform | Format | Target Environments |
| :--- | :--- | :--- |
| **macOS** | **Universal DMG (`.dmg`)** | macOS 11.0+ (Apple Silicon `arm64` & Intel `x86_64`) |
| **Linux (Debian/Ubuntu)** | **Debian Package (`.deb`)** | Ubuntu, Debian, Linux Mint |
| **Linux (RedHat/Enterprise)** | **RPM Package (`.rpm`)** | RHEL, Rocky Linux, AlmaLinux, CentOS, Fedora |
| **Windows** | **Standalone Executable (`.exe`)** | Windows 10 & 11 (64-bit portable, run directly) |

---

## 6. Local Development & Automated Testing

The codebase requires only standard Go toolchains (Go 1.22+):

```bash
# 1. Clone the repository
git clone https://github.com/spanNOAA/verification_studio.git
cd verification_studio

# 2. Run automated test suite
go test -v ./...

# 3. Run application directly
go run ./gui

# 4. Compile local executable
go build -o verification_studio ./gui
```
