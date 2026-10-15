# NOAA GSL Verification Studio

An end-to-end automated verification and presentation generation suite engineered for numerical weather prediction models. By supplying experiment model identifiers and verification date ranges, this tool **automatically evaluates NOAA GSL MATS statistical curves, retrieves remote HPC radar reflectivity graphics**, and **generates professional 23-slide PowerPoint presentations and 1:1 synchronized web presentations with a single click**.

---

## 1. Key Features

1. **Pure Plotly Canvas Export**:
   - Directly calls the underlying Plotly canvas export engine (`Plotly.toImage`).
   - Completely bypasses browser UI elements, navigation bars, controls, and range sliders, preserving only crystal-clear high-definition charts (1761 × 857 resolution).

2. **Consistent Multi-Experiment Styling & Standardized Line Styles**:
   - **Fixed Color per Model**: Each experiment maintains an identical, high-contrast dedicated palette color across all slides.
   - **Standardized Line Typology**: **RMSE is strictly solid**, **BIAS is strictly dashed**.
   - Markers and error bars adaptively inherit their respective experiment's theme color.

3. **Comprehensive Operational Verification Coverage**:
   - **Surface Dieoff Verification**: 2-m Temperature, 2-m Dewpoint, 10-m Wind Speed.
   - **Surface Diurnal Time Series**: 1-h, 3-h, 12-h, and 24-h 6-panel diurnal evolution layouts.
   - **Radar Composite Reflectivity**: Quantitative Dieoff CSI/Bias and 1-h/3-h diurnal evolution at 15 dBZ, 25 dBZ, and 35 dBZ thresholds.
   - **Cloud Ceiling Dieoff**: Aviation threshold verification (< 500 ft, < 1000 ft, < 3000 ft CSI & Bias).
   - **Visibility Verification**: Aviation threshold verification (< 1 mi, < 3 mi, < 5 mi CSI & Bias).
   - **Upper Air Sounding Dieoff (RAOBs)**: Temperature, Wind Speed, Relative Humidity (RHobT).
   - **Aircraft Observations (AMDAR Traditional)**: Temperature, Wind Speed, and Humidity aloft.
   - **Upper Air Vertical Profiles**: 1-h, 3-h, 6-h, and 12-h vertical profiles from 1000 hPa to 100 hPa.
   - **Synoptic Radar Case Comparisons**: Automated side-by-side mosaic layout across severe convective cases (with NSSL MRMS observations).

4. **Universal Graceful Degradation**:
   - Availability pre-checks are executed before retrieval for every verification category.
   - If a specific metric or model subset is unavailable in the database, the engine automatically skips or plots available subsets with clean console `[NOTICE]` logs without crashing the pipeline.

5. **Synchronized Dual-Format Output (PowerPoint + 1:1 Interactive HTML)**:
   - Produces both an editable PPTX deck and a pixel-accurate interactive HTML presentation (`.html`) in the output directory.
   - Supports keyboard arrow navigation, **Slideshow Mode**, **Continuous Scroll Mode**, and **HD Lightbox zoom** for granular inspection.

6. **Smart Caching & Resumable Execution**:
   - Downloaded charts and radar images are cached locally. Re-running the pipeline resumes instantly.

7. **Zero-Config HPC Remote Integration**:
   - Bundles the core `hpc-run` engine for instant remote retrieval of synoptic radar graphics from NOAA HPC clusters (Gaea, Hera, Ursa, Orion, Hercules).
   - If an HPC cluster session is offline, automatically launches an interactive macOS Terminal window for PIN + YubiKey authentication, and cleanly auto-closes the window once connected.

---

## 2. Usage Instructions

### Method 1: Compiled Native Desktop Application (Verification Studio.app — Recommended)

Built with Go and Cocoa WebKit, this lightweight standalone app provides modern UI controls and real-time streaming console logs:

1. **How to Launch**:
   - **Double-click in Finder**:
     - `Verification Studio.app` (Standard macOS Application bundle with icon)
   - **Or drag to `/Applications`** to pin to macOS Dock or Launchpad.
   - **Or launch via terminal**:
     ```bash
     mats_reporter/verification_studio
     ```
2. **Workflow**:
   - **1. Report Title**: Enter or edit the presentation title (e.g., `May 6-13 2024 Retro Runs: Evaluation`).
   - **2. Output Directory**: Type the destination path or click **`📂 Browse...`** to pick a folder.
   - **3. Experiments**: Enter or paste comparison experiments (one per line). Format supports:
     - `Experiment_ID - machine:path` (e.g., `RRFSv2_conus_3km_ret_generictquv - gaeac6:/gpfs/f6/wrfruc/scratch/...`)
     - Or simply `Experiment_ID` (e.g., `RRFSv2_conus_3km_ret_v2x_retv4`)
     - Machines supported: `gaeac5`, `gaeac6`, `gaeac7`, `gaea`, `hera`, `ursa`, `orion`, `hercules`.
   - Click **`🚀 Start Automated Generation`**: Watch real-time streaming progress in the live log console.

---

### Method 2: Configuration File Mode (YAML)

Edit `config_example.yaml`:
```yaml
title: "May 6-13 2024 Retro Runs: Surface DA & Terrain Error Evaluation"
models:
  - id: "RRFSv2_conus_3km_ret_generictquv"
    name: "generictquv"
    color: "rgb(255, 0, 0)"
  - id: "RRFSv2_conus_3km_ret_halfTerr"
    name: "halfTerr"
    color: "rgb(0, 0, 255)"
  - id: "RRFSv2_conus_3km_ret_RRFSv2X_retv3"
    name: "retv3"
    color: "rgb(255, 165, 0)"
  - id: "RRFSv2_conus_3km_ret_v2x_retv4"
    name: "retv4"
    color: "rgb(128, 128, 128)"

dates:
  surface: "05/06/2024 10:00 - 05/13/2024 00:00"
  upperair: "05/06/2024 00:00 - 05/13/2024 12:00"

output_dir: "./output"
output_pptx: "./output/RRFSv2_Verification_Report.pptx"
```

Run via Python:
```bash
python3 mats_reporter.py --config config_example.yaml
```

---

### Method 3: Command-Line Interface (CLI)

```bash
python3 mats_reporter.py \
  --models "generictquv:RRFSv2_conus_3km_ret_generictquv" \
           "halfTerr:RRFSv2_conus_3km_ret_halfTerr" \
           "retv3:RRFSv2_conus_3km_ret_RRFSv2X_retv3" \
           "retv4:RRFSv2_conus_3km_ret_v2x_retv4" \
  --surface-dates "05/06/2024 10:00 - 05/13/2024 00:00" \
  --upperair-dates "05/06/2024 00:00 - 05/13/2024 12:00" \
  --output-dir "./my_run" \
  --output-pptx "./my_run/Report.pptx"
```

---

### CLI Parameter Reference

- `--title`: Presentation title displayed on slides and header bar.
- `--models`: List of models with optional aliases and paths.
- `--output-dir`: Directory where downloaded charts and assets are stored.
- `--output-pptx`: Destination file path for the PowerPoint presentation.
- `--output-html`: Destination file path for the HTML presentation (defaults to same name `.html`).
- `--skip-fetch`: Skip chart downloads and rebuild PPTX/HTML directly from existing cached images.
- `--skip-pptx`: Generate HTML and fetch images without creating the PPTX file.
- `--skip-html`: Skip generation of the HTML presentation.
- `--skip-cref`: Skip radar composite reflectivity comparison slides.
