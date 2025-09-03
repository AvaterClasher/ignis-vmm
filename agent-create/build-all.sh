#!/bin/bash
set -euo pipefail
set -x

# Builds kernel and multiple rootfs variants, then moves artifacts to project root

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

pushd "$SCRIPT_DIR" >/dev/null

# Ensure agent directory exists in repo root
mkdir -p "$REPO_ROOT/agent"

# Build kernel
rm -f "$REPO_ROOT/linux/vmlinux"
./build-kernel.sh

# Build agent binary (static)
OUTPUT_BIN=agent ./build-static.sh

# Build rootfs variants
VARIANTS=(python go)
for v in "${VARIANTS[@]}"; do
  ROOTFS_NAME="rootfs-$v"
  VARIANT="$v" ./build-rootfs.sh
  rm -f "$REPO_ROOT/agent/rootfs-$v.ext4"
  mv "rootfs-$v.ext4" "$REPO_ROOT/agent/"
done

popd >/dev/null

echo "Artifacts:"
echo "- Kernel: $REPO_ROOT/linux/vmlinux"
echo "- Rootfs: $REPO_ROOT/agent/rootfs-python.ext4"
echo "- Rootfs: $REPO_ROOT/agent/rootfs-go.ext4"