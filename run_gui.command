#!/bin/bash
set -e

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
cd "$DIR"

if [ ! -f "$DIR/verification_studio" ]; then
  echo ">>> Binary not found. Building Verification Studio native universal binary..."
  ./build.sh
fi

exec "$DIR/verification_studio" "$@"
