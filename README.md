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
6. **Unified Dual-Mode (Desktop GUI + Headless CLI)**:
   - Launch as a native macOS desktop GUI app or run headlessly in CI/CD pipelines and HPC batch scripts using the exact same binary.

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

## 🚀 3. Usage Guide

### Method 1: Native macOS Desktop Application (Recommended)

1. **Download & Install**:
   - Download [`Verification_Studio_macOS_Universal.dmg`](https://github.com/spanNOAA/verification_studio/releases/latest)
   - Open the DMG and drag **`Verification Studio.app`** to your `/Applications` folder.
2. **Run**:
   - Double-click **Verification Studio** from Launchpad or Finder.
3. **Configure & Start**:
   - **Report Title**: e.g., `RRFSv2 Model Verification & Performance Evaluation`
   - **Output Directory**: Click `📂 Browse...` to pick a destination folder.
   - **Models**: Enter or paste experiments (one per line):
     ```text
     retv4:RRFSv2_conus_3km_ret_v2x_retv4 - gaeac6:/gpfs/f6/wrfruc/scratch/my_run/cref
     generictquv:RRFSv2_conus_3km_ret_generictquv
     halfTerr:RRFSv2_conus_3km_ret_halfTerr
     ```
   - Click **`🚀 Start Automated Generation`** and watch real-time progress.

---

### Method 2: Headless CLI Mode

The exact same binary supports headless command-line execution for scripts and automated jobs:

```bash
./verification_studio \
  --models "retv4:RRFSv2_conus_3km_ret_v2x_retv4,generictquv:RRFSv2_conus_3km_ret_generictquv" \
  --sfc-dates "05/06/2024 10:00 - 05/13/2024 00:00" \
  --ua-dates "05/06/2024 00:00 - 05/13/2024 12:00" \
  --regions "all,east,west" \
  --output-dir "~/Desktop/my_verification" \
  --report-name "RRFSv2_May2024_Report"
```

---

### Method 3: YAML / JSON Configuration File Mode

Prepare a `config.yaml` (see [`config_example.yaml`](config_example.yaml)):

```yaml
title: "RRFSv2 Model Verification & Performance Evaluation"

models:
  - id: "RRFSv2_conus_3km_ret_generictquv"
    name: "generictquv"
  - id: "RRFSv2_conus_3km_ret_halfTerr"
    name: "halfTerr"
  - id: "RRFSv2_conus_3km_ret_v2x_retv4"
    name: "retv4"
    cluster: "gaeac6"
    path: "/gpfs/f6/wrfruc/scratch/my_run/cref"

dates:
  surface: "05/06/2024 10:00 - 05/13/2024 00:00"
  upperair: "05/06/2024 00:00 - 05/13/2024 12:00"

regions:
  - "all"
  - "east"

output_dir: "~/Desktop/Verification_Output"
report_name: "RRFSv2_Verification_Report"
```

Execute with a single command:
```bash
./verification_studio --config config.yaml
```

---

## 🛠️ 4. CLI Parameter Reference

| Flag | Shorthand | Description | Default |
| :--- | :--- | :--- | :--- |
| `--config` | `-c` | Path to YAML or JSON configuration file | `""` |
| `--title` | | Presentation title displayed on cover and slides | `"RRFSv2 Model Verification..."` |
| `--output-dir`| `-o` | Target directory for generated PPT, HTML, and charts | *(Required)* |
| `--report-name`| | Base name for output `.pptx` and `.html` files | `"mats_report"` |
| `--models` | | Evaluated model specifications (comma-separated or text) | `""` |
| `--sfc-dates` | | Date range for surface & METAR evaluations | `"05/06/2024 10:00 - ..."` |
| `--ua-dates` | | Date range for upper-air RAOBs & AMDAR soundings | `"05/06/2024 00:00 - ..."` |
| `--regions` | | Evaluated domains (`all`, `east`, `west`) | `"all"` |
| `--skip-fetch` | | Skip chart downloads; rebuild reports from local cache | `false` |
| `--force-fetch`| | Force fresh downloads even if cache exists | `false` |
| `--include-cref`| | Include synoptic radar composite reflectivity comparison | `false` |
| `--help` | `-h` | Display help and usage instructions | |
| `--version` | `-v` | Print version information | |

---

## 🔒 5. HPC Remote Integration (`hpc-tools`)

Verification Studio incorporates `hpc-run` for secure remote access to NOAA supercomputers:
- **Supported Clusters**: NOAA RDHPCS Gaea (`gaeac5`, `gaeac6`, `gaeac7`), Hera (`hera`), Ursa (`ursa`), and partners (MSU `orion`, `hercules`).
- **Read-Only Security**: Strict minimal API (list, stat, download). Arbitrary commands, write, and job submissions are structurally prohibited.
- **YubiKey / SmartCard Support**: If an HPC session is offline, the studio prompts for one-time interactive PIN + YubiKey authentication in a dedicated terminal window.

---

## 📦 6. Building from Source

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
