#!/bin/bash
set -euo pipefail
script_dirpath="$(cd "$(dirname "${0}")" && pwd)"

# Create build directory if it doesn't exist
mkdir -p "${script_dirpath}/build"

# If no GOOS/GOARCH specified, build for current platform
if [[ -z "${GOOS:-}" && -z "${GOARCH:-}" ]]; then
    echo "Building opwriting for current platform..."
    go build -o "${script_dirpath}/build/opwriting" "${script_dirpath}"
    echo "Build complete: ${script_dirpath}/build/opwriting"
else
    # Build for specified platform
    output_name="opwriting-${GOOS}-${GOARCH}"
    
    echo "Building ${output_name} for ${GOOS}-${GOARCH}..."
    go build -o "${script_dirpath}/build/${output_name}" "${script_dirpath}"
    echo "Build complete: ${script_dirpath}/build/${output_name}"
fi