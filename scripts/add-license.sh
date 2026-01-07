#!/bin/bash
# Add Apache 2.0 license headers to all Go source files that are missing them

set -e

# License header pattern to check for (SPDX short form)
LICENSE_PATTERN="SPDX-License-Identifier: Apache-2.0"

# License header to add
LICENSE_HEADER="// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

"

# Find all Go files (excluding vendor, generated files, etc.)
GO_FILES=$(find . -name "*.go" -not -path "./vendor/*" -not -path "./.git/*" -type f)

ADDED_COUNT=0

for file in $GO_FILES; do
    # Check if the file contains the license identifier in the first 10 lines
    if ! head -n 10 "$file" | grep -q "$LICENSE_PATTERN"; then
        echo "Adding license header to: $file"
        
        # Create temp file with license header + original content
        tmp_file=$(mktemp)
        echo -n "$LICENSE_HEADER" > "$tmp_file"
        cat "$file" >> "$tmp_file"
        mv "$tmp_file" "$file"
        
        ADDED_COUNT=$((ADDED_COUNT + 1))
    fi
done

if [ $ADDED_COUNT -gt 0 ]; then
    echo ""
    echo "Added license headers to $ADDED_COUNT file(s)."
else
    echo "All Go files already have license headers."
fi


