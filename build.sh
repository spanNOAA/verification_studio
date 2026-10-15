#!/bin/bash
set -e

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
cd "$DIR"

echo "=================================================================="
echo " Building MATS Studio Universal Binary (macOS arm64 + amd64)..."
echo "=================================================================="

cd "$DIR/gui"
CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build -o ../mats_studio_arm64 .
CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build -o ../mats_studio_amd64 .

cd "$DIR"
lipo -create -output mats_studio mats_studio_arm64 mats_studio_amd64
rm -f mats_studio_arm64 mats_studio_amd64
codesign --force --sign - mats_studio
chmod +x mats_studio

echo ""
echo "[SUCCESS] Built universal binary: $DIR/mats_studio"
file mats_studio
