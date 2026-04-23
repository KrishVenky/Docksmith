#!/usr/bin/env bash
set -euo pipefail

BASE_IMAGE="${BASE_IMAGE:-alpine:3.19}"
BASE_TAR="${BASE_TAR:-alpine-minirootfs-3.19.1-x86_64.tar}"
DEMO_IMAGE="${DEMO_IMAGE:-demo:v1}"

if [[ ! -f "$BASE_TAR" ]]; then
  echo "Missing $BASE_TAR in $(pwd)"
  exit 1
fi

find . -name "*.o" -delete
rm -f docksmith docksmith_c

make clean
make

sudo ./docksmith import "$BASE_IMAGE" "$BASE_TAR"
sudo ./docksmith build --no-cache -t "$DEMO_IMAGE" -f Docksmithfile .
sudo ./docksmith run -e GREETING=hello "$DEMO_IMAGE"

echo
echo "--- Images ---"
sudo ./docksmith images
echo
echo "--- Cache ---"
sudo ./docksmith cache
echo
echo "--- Store tree ---"
find "$HOME/.docksmith" -maxdepth 3 -type f | sort