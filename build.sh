#!/bin/bash
set -e

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
cd "$DIR"

echo "=================================================================="
echo " Building Verification Studio Universal Binary (macOS arm64 + amd64)..."
echo "=================================================================="

cd "$DIR/gui"
CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build -o ../verification_studio_arm64 .
CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build -o ../verification_studio_amd64 .

cd "$DIR"
lipo -create -output verification_studio verification_studio_arm64 verification_studio_amd64
rm -f verification_studio_arm64 verification_studio_amd64
codesign --force --sign - verification_studio
chmod +x verification_studio
ln -sf verification_studio mats_studio

echo ""
echo "[SUCCESS] Built universal binary: $DIR/verification_studio"
file verification_studio
