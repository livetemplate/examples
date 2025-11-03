#!/bin/bash

# Test script for all LiveTemplate examples
# Usage: ./test-all.sh [--skip-disabled]

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Track results
declare -a PASSED=()
declare -a FAILED=()
declare -a SKIPPED=()

# Working examples
WORKING_EXAMPLES=(
    "counter"
    "chat"
    "todos"
    "graceful-shutdown"
    "testing/01_basic"
)

# Disabled examples (require internal/observe package)
DISABLED_EXAMPLES=(
    "observability"
    "production/single-host"
    "trace-correlation"
)

echo "================================================"
echo "  LiveTemplate Examples Test Suite"
echo "================================================"
echo ""

# Parse arguments
SKIP_DISABLED=false
if [[ "$1" == "--skip-disabled" ]]; then
    SKIP_DISABLED=true
fi

# Test a single example
test_example() {
    local example=$1
    local is_disabled=$2

    echo "Testing: $example"
    echo "----------------------------------------"

    if [[ ! -d "$example" ]]; then
        echo -e "${RED}✗ Directory not found: $example${NC}"
        FAILED+=("$example")
        return 1
    fi

    cd "$example"

    # Download dependencies
    echo "  → Downloading dependencies..."
    if ! go mod download 2>&1 | sed 's/^/    /'; then
        echo -e "${RED}✗ Failed to download dependencies${NC}"
        cd - > /dev/null
        FAILED+=("$example")
        return 1
    fi

    # Build
    echo "  → Building..."
    if ! go build -v . 2>&1 | sed 's/^/    /'; then
        echo -e "${RED}✗ Build failed${NC}"
        cd - > /dev/null
        FAILED+=("$example")
        return 1
    fi

    # Skip tests for disabled examples unless explicitly requested
    if [[ "$is_disabled" == "true" ]] && [[ "$SKIP_DISABLED" == "true" ]]; then
        echo -e "${YELLOW}⊘ Tests skipped (disabled example)${NC}"
        cd - > /dev/null
        SKIPPED+=("$example")
        return 0
    fi

    # Run tests
    echo "  → Running tests..."
    if go test -v -race -timeout=5m ./... 2>&1 | sed 's/^/    /'; then
        echo -e "${GREEN}✓ All tests passed${NC}"
        cd - > /dev/null
        PASSED+=("$example")
        return 0
    else
        echo -e "${RED}✗ Tests failed${NC}"
        cd - > /dev/null
        FAILED+=("$example")
        return 1
    fi
}

# Test all working examples
echo ""
echo "Testing working examples..."
echo "================================================"
echo ""

for example in "${WORKING_EXAMPLES[@]}"; do
    test_example "$example" "false"
    echo ""
done

# Test disabled examples
if [[ "$SKIP_DISABLED" == "false" ]]; then
    echo ""
    echo "Testing disabled examples (require internal/observe)..."
    echo "================================================"
    echo ""

    for example in "${DISABLED_EXAMPLES[@]}"; do
        test_example "$example" "true"
        echo ""
    done
fi

# Print summary
echo ""
echo "================================================"
echo "  Test Summary"
echo "================================================"
echo ""

if [ ${#PASSED[@]} -gt 0 ]; then
    echo -e "${GREEN}✓ Passed (${#PASSED[@]}):${NC}"
    for example in "${PASSED[@]}"; do
        echo "    - $example"
    done
    echo ""
fi

if [ ${#SKIPPED[@]} -gt 0 ]; then
    echo -e "${YELLOW}⊘ Skipped (${#SKIPPED[@]}):${NC}"
    for example in "${SKIPPED[@]}"; do
        echo "    - $example"
    done
    echo ""
fi

if [ ${#FAILED[@]} -gt 0 ]; then
    echo -e "${RED}✗ Failed (${#FAILED[@]}):${NC}"
    for example in "${FAILED[@]}"; do
        echo "    - $example"
    done
    echo ""
    echo "================================================"
    exit 1
fi

echo "================================================"
echo -e "${GREEN}All tests passed!${NC}"
echo "================================================"
exit 0
