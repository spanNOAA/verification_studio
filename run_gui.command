#!/bin/bash
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
cd "$DIR"

echo "=================================================================="
echo " Starting NOAA GSL MATS Verification Studio (Compiled Native App)..."
echo "=================================================================="

"$DIR/mats_studio"
