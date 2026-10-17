#!/bin/bash
set -e

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
cd "$DIR"

echo "=================================================================="
echo " Building NOAA GSL Verification Studio Universal (arm64 + x86_64)..."
echo "=================================================================="

BUILD_DIR="$DIR/build"
mkdir -p "$BUILD_DIR"

cd "$DIR/gui"
echo "[1/4] Compiling macOS arm64 binary..."
CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build -o "$BUILD_DIR/verification_studio_arm64" .

echo "[2/4] Compiling macOS amd64 (x86_64) binary..."
CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build -o "$BUILD_DIR/verification_studio_amd64" .

cd "$DIR"
echo "[3/4] Creating Universal Mach-O binary via lipo..."
lipo -create -output "$BUILD_DIR/verification_studio" "$BUILD_DIR/verification_studio_arm64" "$BUILD_DIR/verification_studio_amd64"
rm -f "$BUILD_DIR/verification_studio_arm64" "$BUILD_DIR/verification_studio_amd64"
codesign --force --sign - "$BUILD_DIR/verification_studio"
chmod +x "$BUILD_DIR/verification_studio"

APP_PATH="/Users/sijie.pan/Desktop/tmp/Verification Studio.app"
DMG_PATH="/Users/sijie.pan/Desktop/tmp/Verification_Studio_macOS_Universal.dmg"

if [ -d "$APP_PATH" ]; then
    echo "[4/4] Updating macOS Application Bundle & Signing..."
    cp "$BUILD_DIR/verification_studio" "$APP_PATH/Contents/MacOS/verification_studio"
    codesign --force --deep --sign - "$APP_PATH"

    echo "Rebuilding Universal DMG installer..."
    rm -f "$DMG_PATH"
    hdiutil create -volname "Verification Studio" -srcfolder "$APP_PATH" -ov -format UDZO "$DMG_PATH" >/dev/null 2>&1
fi

rm -rf "$BUILD_DIR"

echo ""
echo "=================================================================="
echo " [SUCCESS] Verification Studio Universal App successfully built!"
echo "   • Application : $APP_PATH"
if [ -f "$DMG_PATH" ]; then
    echo "   • DMG Package : $DMG_PATH"
fi
echo "=================================================================="
