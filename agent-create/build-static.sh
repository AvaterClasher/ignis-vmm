#!/bin/bash

set -xe

export GIN_MODE=release
OUTPUT_BIN=${OUTPUT_BIN:-agent}
go build -tags netgo -ldflags '-extldflags "-static"' -o "$OUTPUT_BIN"
