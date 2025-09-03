#!/bin/bash

set -xe

VARIANT=${VARIANT:-base}
ROOTFS_SIZE_MB=${ROOTFS_SIZE_MB:-1000}
ROOTFS_NAME=${ROOTFS_NAME:-rootfs}
ROOTFS_FILE="${ROOTFS_NAME}-${VARIANT}.ext4"

dd if=/dev/zero of="$ROOTFS_FILE" bs=1M count="$ROOTFS_SIZE_MB"
mkfs.ext4 "$ROOTFS_FILE"
mkdir -p /tmp/my-rootfs
mount "$ROOTFS_FILE" /tmp/my-rootfs

EXTRA_PACKAGES=""
case "$VARIANT" in
    python)
        EXTRA_PACKAGES="python3"
        ;;
    go)
        EXTRA_PACKAGES="go"
        ;;
    base)
        EXTRA_PACKAGES=""
        ;;
    *)
        echo "Unknown VARIANT: $VARIANT" >&2
        exit 1
        ;;
esac

docker run -i --rm \
    -e EXTRA_PACKAGES="$EXTRA_PACKAGES" \
    -v /tmp/my-rootfs:/my-rootfs \
    -v "$(pwd)/agent:/usr/local/bin/agent" \
    -v "$(pwd)/openrc-run.sh:/etc/init.d/agent" \
    alpine sh <alpine-setup.sh

umount /tmp/my-rootfs
