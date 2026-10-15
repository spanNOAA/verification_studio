#!/bin/bash
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
cd "$DIR"

echo "=================================================================="
echo " Starting NOAA GSL Verification Studio (Compiled Native App)..."
echo "=================================================================="

if [ -f "$DIR/verification_studio" ]; then
  "$DIR/verification_studio"
else
  "$DIR/mats_studio"
fi
