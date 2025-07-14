#!/bin/bash
set -euo pipefail
script_dirpath="$(cd "$(dirname "${0}")" && pwd)"

# Create build directory if it doesn't exist
mkdir -p "${script_dirpath}/build"

# Build the Go binary
echo "Building opwriting..."
go build -o "${script_dirpath}/build/opwriting" "${script_dirpath}"

echo "Build complete: ${script_dirpath}/build/opwriting"