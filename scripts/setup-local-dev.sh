#!/bin/bash
#
# Setup local development for all examples with sibling repositories
#
# This script configures all example applications to use local checkouts
# of the core livetemplate library and lvt CLI instead of published versions.
#
# Usage:
#   ./scripts/setup-local-dev.sh          # Enable local development
#   ./scripts/setup-local-dev.sh --undo   # Revert to published versions
#
# Requirements:
#   - Core library must be checked out at ../livetemplate/
#   - LVT must be checked out at ../lvt/
#   - All repos should be sibling directories under the same parent

set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

CORE_PATH="../livetemplate"
LVT_PATH="../lvt"
CORE_MODULE="github.com/livetemplate/livetemplate"
LVT_MODULE="github.com/livetemplate/lvt"

# Working examples (disabled examples excluded)
WORKING_EXAMPLES=(
    "counter"
    "chat"
    "todos"
    "graceful-shutdown"
    "testing/01_basic"
)

# Check if we're undoing the setup
if [[ "$1" == "--undo" || "$1" == "undo" ]]; then
    echo "Reverting all examples to published versions..."
    echo ""

    for example in "${WORKING_EXAMPLES[@]}"; do
        if [ ! -d "$example" ]; then
            echo -e "${YELLOW}⚠ Skipping $example (directory not found)${NC}"
            continue
        fi

        echo "  → Reverting $example..."
        cd "$example"
        go mod edit -dropreplace="$CORE_MODULE"
        go mod edit -dropreplace="$LVT_MODULE"
        go mod tidy
        cd - > /dev/null
    done

    echo ""
    echo -e "${GREEN}✓ All examples reverted to published versions${NC}"
    echo ""
    echo "To re-enable local development, run: ./scripts/setup-local-dev.sh"
    exit 0
fi

# Check if core library exists
if [ ! -d "$CORE_PATH" ]; then
    echo -e "${RED}✗ Core library not found at $CORE_PATH${NC}"
    echo ""
    echo "Expected directory structure:"
    echo "  parent/"
    echo "  ├── livetemplate/  (core library)"
    echo "  ├── lvt/           (CLI tool)"
    echo "  └── examples/      (this repository)"
    echo ""
    echo "Please clone the core library:"
    echo "  git clone git@github.com:livetemplate/livetemplate.git $CORE_PATH"
    exit 1
fi

# Check if core library is valid
if [ ! -f "$CORE_PATH/go.mod" ]; then
    echo -e "${RED}✗ $CORE_PATH does not appear to be a valid Go module${NC}"
    exit 1
fi

# Check if LVT exists (optional but recommended)
LVT_EXISTS=false
if [ -d "$LVT_PATH" ] && [ -f "$LVT_PATH/go.mod" ]; then
    LVT_EXISTS=true
    echo -e "${GREEN}✓ Found LVT at $LVT_PATH${NC}"
else
    echo -e "${YELLOW}⚠ LVT not found at $LVT_PATH${NC}"
    echo "  Examples will use published LVT version"
    echo "  To use local LVT, clone it:"
    echo "    git clone git@github.com:livetemplate/lvt.git $LVT_PATH"
    echo ""
fi

echo "Setting up local development for all examples..."
echo "  Core library: $CORE_PATH"
if [ "$LVT_EXISTS" = true ]; then
    echo "  LVT CLI:      $LVT_PATH"
fi
echo ""

# Setup each example
SUCCESS_COUNT=0
SKIP_COUNT=0

for example in "${WORKING_EXAMPLES[@]}"; do
    if [ ! -d "$example" ]; then
        echo -e "${YELLOW}⚠ Skipping $example (directory not found)${NC}"
        ((SKIP_COUNT++))
        continue
    fi

    echo "  → Setting up $example..."
    cd "$example"

    # Add replace directive for core library
    go mod edit -replace="$CORE_MODULE=../../livetemplate"

    # Add replace directive for LVT if available
    if [ "$LVT_EXISTS" = true ]; then
        go mod edit -replace="$LVT_MODULE=../../lvt"
    fi

    go mod tidy
    cd - > /dev/null
    ((SUCCESS_COUNT++))
done

echo ""
echo -e "${GREEN}✓ Local development setup complete!${NC}"
echo ""
echo "Summary:"
echo "  ✓ Configured: $SUCCESS_COUNT examples"
if [ $SKIP_COUNT -gt 0 ]; then
    echo "  ⊘ Skipped:    $SKIP_COUNT examples"
fi
echo ""
echo "All working examples now use the local core library"
if [ "$LVT_EXISTS" = true ]; then
    echo "and local LVT from sibling directories."
else
    echo "from the sibling directory."
fi
echo ""
echo "Changes you make to the core library (or LVT) will be"
echo "immediately reflected when building or testing examples."
echo ""
echo "To revert to published versions, run:"
echo "  ./scripts/setup-local-dev.sh --undo"
