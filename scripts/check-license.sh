#!/bin/bash
# Check that all Go source files have Apache 2.0 license headers

set -e

# License header pattern to check for (SPDX short form)
LICENSE_PATTERN="SPDX-License-Identifier: Apache-2.0"

# Find all Go files (excluding vendor, generated files, etc.)
GO_FILES=$(find . -name "*.go" -not -path "./vendor/*" -not -path "./.git/*" -type f)

MISSING_HEADER=()

for file in $GO_FILES; do
    # Check if the file contains the license identifier in the first 10 lines
    if ! head -n 10 "$file" | grep -q "$LICENSE_PATTERN"; then
        MISSING_HEADER+=("$file")
    fi
done

if [ ${#MISSING_HEADER[@]} -gt 0 ]; then
    echo "The following files are missing Apache 2.0 license headers:"
    echo ""
    for file in "${MISSING_HEADER[@]}"; do
        echo "  $file"
    done
    echo ""
    echo "Please add the following header to the top of each file:"
    echo ""
    echo "  // Copyright $(date +%Y) Elasticsearch B.V."
    echo "  // SPDX-License-Identifier: Apache-2.0"
    echo ""
    exit 1
fi

echo "All Go files have proper license headers."


