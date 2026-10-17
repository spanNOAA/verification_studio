# NOAA GSL Verification Studio

[![GitHub Release](https://img.shields.io/github/v/release/spanNOAA/verification_studio?color=blue&label=Latest%20Release)](https://github.com/spanNOAA/verification_studio/releases/latest)
[![macOS Universal](https://img.shields.io/badge/Platform-macOS%20(Apple%20Silicon%20%2B%20Intel)-brightgreen.svg)](https://github.com/spanNOAA/verification_studio/releases/latest)
[![Binary Size](https://img.shields.io/badge/Binary%20Size-16%20MB-blue.svg)](https://github.com/spanNOAA/verification_studio/releases/latest)
[![DMG Size](https://img.shields.io/badge/DMG%20Installer-9.3%20MB-success.svg)](https://github.com/spanNOAA/verification_studio/releases/latest)
[![License](https://img.shields.io/badge/License-NOAA%20GSL-informational.svg)](https://github.com/spanNOAA/verification_studio)

An end-to-end automated verification and presentation generation suite engineered for numerical weather prediction models (e.g. RRFS, HRRR). By specifying experiment model identifiers and verification date ranges, Verification Studio **automatically evaluates NOAA GSL MATS statistical curves, retrieves remote HPC radar reflectivity graphics**, and **generates professional 26-slide PowerPoint presentations (.pptx) and 1:1 synchronized interactive web presentations (.html) with a single click**.

---

## ⚡ 1. Modern Architecture (Pure Go Engine)

Verification Studio v1.0 has been re-architected from the ground up into a **100% pure Go standalone application**:

1. **Zero Python / Node.js Dependencies**:
   - The entire verification engine, HTTP automation, Plotly image extraction, PowerPoint compiler, and user interface are written in Go.
   - No Python runtime, `pip`, virtual environments, or heavy third-party packages required.
2. **Zero Bundled Browsers**:
   - Directly reuses host system browsers (**Google Chrome**, **Microsoft Edge**, **Brave**, **Chromium**, or **Arc**) via native Chrome DevTools Protocol (CDP).
   - Eliminates 400MB+ bundled headless browser downloads.
3. **Ultra-Lightweight Footprint**:
   - Standalone application bundle reduced from 471 MB to **16 MB**.
   - Universal DMG installer reduced from 222 MB to **9.3 MB**.
4. **Enhanced HD Font & Chart Layout**:
   - Dynamic layout injection enhances Plotly canvas font sizes (Title 26px, Axes 20px, Ticks/Legend 16px).
   - Charts remain crystal-clear, bold, and comfortably readable when scaled into 3-panel slides.
5. **Multi-Domain Regional Evaluation**:
   - Seamlessly generates Full Domain (`all`), Eastern Region (`east`), and Western Complex Terrain (`west`) verification decks.
   - Interactive HTML report features instantaneous region switching tabs.
6. **Native macOS Desktop Experience**:
   - Modern, clean native user interface with embedded real-time progress bars, live log console, and one-click folder browsing.

---

## 📊 2. Verification Coverage

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

## 🚀 3. Usage Guide (Native macOS Desktop App)

1. **Download & Install**:
   - Download [`Verification_Studio_macOS_Universal.dmg`](https://github.com/spanNOAA/verification_studio/releases/latest)
   - Open the DMG and drag **`Verification Studio.app`** to your `/Applications` folder.
2. **Launch Application**:
   - Double-click **Verification Studio** from Launchpad, Finder, or `/Applications`.
3. **Configure Settings**:
   - **Report Title**: e.g., `RRFSv2 Model Verification & Performance Evaluation`
   - **Output Directory**: Click `📂 Browse...` to pick a destination folder on your Desktop.
   - **Models**: Enter or paste experiments (one per line):
     ```text
     retv4:RRFSv2_conus_3km_ret_v2x_retv4 - gaeac6:/gpfs/f6/wrfruc/scratch/my_run/cref
     generictquv:RRFSv2_conus_3km_ret_generictquv
     halfTerr:RRFSv2_conus_3km_ret_halfTerr
     ```
   - **Date Ranges**: Specify Surface (METAR) and Upper-Air (RAOBs/AMDAR) date ranges.
   - **Evaluated Regions**: Toggle Full Domain (`all`), Eastern (`east`), and Western Complex Terrain (`west`).
4. **Generate**:
   - Click **`🚀 Start Automated Generation`** and monitor the live dual-tier progress bar and output logs.
   - Upon completion, synchronized `.pptx` and `.html` presentation decks are instantly available in your destination directory.

---

## 🔒 4. Native Pure Go HPC Integration (`RO-v1` Engine)

Verification Studio includes a **built-in, native Pure Go HPC read-only engine** for secure, self-contained remote access to NOAA supercomputers with zero external dependencies:

- **Supported Supercomputers**: NOAA RDHPCS Gaea (`gaea-c5`, `gaea-c6`, `gaea-c7`), Hera (`hera`), Ursa (`ursa`), and university partners (MSU `orion`, `hercules`).
- **Zero Python / Zero Dependencies**: Fully implemented in pure Go. Remote radar retrieval works out-of-the-box on distributed systems without requiring Python, scripts, or package installations.
- **Strict Read-Only Security Compliance (RO-v1)**:
  - Strict capability whitelist: only `list`, `stat`, and `download` operations are structurally permitted.
  - Arbitrary remote shell command execution, file writing, modification, deletion, and batch job submissions (`sbatch`, `qsub`) are permanently blocked by the protocol daemon.
  - Local Unix domain sockets operate under strict `0600` permissions with peer process UID verification.
  - Safe atomic file replacement with symlink traversal protection and 50 MB download payload limits.
- **Zero-Config Fallback**:
  - Automatically resolves hostnames to official Canonical FQDNs if `~/.ssh/config` is unconfigured.
  - Prompts for interactive username confirmation with candidate default, persisting confirmed credentials to `~/.hpc_user_<cluster>`.
- **Automatic Session Teardown**:
  - Background HPC session tunnels automatically disconnect and clean up socket files upon verification report completion, preventing persistent lingering connections.
- **Built-in HPC Tools**:
  ```bash
  # Check cluster connection status
  ./verification_studio hpc-run --status

  # Authenticate PIN + YubiKey interactively
  ./verification_studio hpc-auth gaea-c6

  # Standalone read-only operations
  ./verification_studio hpc-run gaea-c6 list /gpfs/f6/scratch1
  ./verification_studio hpc-run gaea-c6 download /path/to/cref.png ./local.png
  ```

---

## 📦 5. Building from Source

To compile the universal macOS binary from source:

```bash
# 1. Clone the repository
git clone https://github.com/spanNOAA/verification_studio.git
cd verification_studio

# 2. Build macOS Universal Binary (arm64 + x86_64)
./build.sh

# 3. Verify output
./verification_studio --version
```
