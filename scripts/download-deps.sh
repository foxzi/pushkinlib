#!/bin/bash
#
# Download external JavaScript dependencies for offline use
#

set -e

VENDOR_DIR="web/static/vendor"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$PROJECT_ROOT"

echo "📦 Downloading external dependencies..."

# Create vendor directory
mkdir -p "$VENDOR_DIR"

# Dependencies configuration
declare -A DEPS=(
    ["vue.global.js"]="https://unpkg.com/vue@3/dist/vue.global.js"
    ["axios.min.js"]="https://unpkg.com/axios/dist/axios.min.js"
)

# Download each dependency
for filename in "${!DEPS[@]}"; do
    url="${DEPS[$filename]}"
    output="$VENDOR_DIR/$filename"

    echo "  → Downloading $filename from $url"

    if command -v curl &> /dev/null; then
        curl -L "$url" -o "$output"
    elif command -v wget &> /dev/null; then
        wget "$url" -O "$output"
    else
        echo "❌ Error: Neither curl nor wget is available"
        exit 1
    fi

    if [ -f "$output" ]; then
        size=$(stat -f%z "$output" 2>/dev/null || stat -c%s "$output" 2>/dev/null)
        echo "  ✓ Downloaded $filename (${size} bytes)"
    else
        echo "  ❌ Failed to download $filename"
        exit 1
    fi
done

echo ""
echo "✅ All dependencies downloaded successfully to $VENDOR_DIR"
echo ""
echo "Dependencies:"
for filename in "${!DEPS[@]}"; do
    echo "  - $filename"
done