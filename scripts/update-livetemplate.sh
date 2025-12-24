#!/bin/bash
set -euo pipefail

# Script to update examples to the latest livetemplate version and create a PR
# Usage: ./scripts/update-livetemplate.sh [--dry-run]

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MODULE="github.com/livetemplate/livetemplate"
DRY_RUN=false

# Disable parent go.work if present
export GOWORK=off

# Parse arguments
for arg in "$@"; do
    case $arg in
        --dry-run)
            DRY_RUN=true
            shift
            ;;
    esac
done

log() {
    echo "[update-livetemplate] $*"
}

error() {
    echo "[update-livetemplate] ERROR: $*" >&2
    exit 1
}

# Get the latest version from GitHub releases
get_latest_version() {
    local latest
    latest=$(gh release view --repo livetemplate/livetemplate --json tagName -q '.tagName' 2>/dev/null)
    if [[ -z "$latest" ]]; then
        error "Failed to fetch latest version from GitHub releases"
    fi
    echo "$latest"
}

# Get current version from go.mod
get_current_version() {
    grep "${MODULE}" "$REPO_ROOT/go.mod" | grep -v "// indirect" | head -1 | awk '{print $2}'
}

# Main
main() {
    log "Checking for livetemplate updates..."

    # Check prerequisites
    if ! command -v gh &> /dev/null; then
        error "gh CLI is required but not installed"
    fi

    if ! command -v go &> /dev/null; then
        error "go is required but not installed"
    fi

    cd "$REPO_ROOT"

    # Get versions
    local latest_version
    latest_version=$(get_latest_version)
    log "Latest version: $latest_version"

    local current_version
    current_version=$(get_current_version)
    log "Current version: $current_version"

    # Compare versions
    if [[ "$current_version" == "$latest_version" ]]; then
        log "Already up to date!"
        exit 0
    fi

    log "New version available: $current_version -> $latest_version"

    if [[ "$DRY_RUN" == "true" ]]; then
        log "[DRY RUN] Would update go.mod from $current_version to $latest_version"
        exit 0
    fi

    # Ensure we're on main and up to date
    git checkout main
    git pull origin main

    # Create a new branch
    local branch_name="chore/upgrade-livetemplate-to-${latest_version}"
    log "Creating branch: $branch_name"

    # Check if branch already exists
    if git show-ref --verify --quiet "refs/heads/${branch_name}"; then
        log "Branch $branch_name already exists, deleting..."
        git branch -D "$branch_name"
    fi

    git checkout -b "$branch_name"

    # Update go.mod
    log "Updating go.mod to $latest_version..."
    go get "${MODULE}@${latest_version}"
    go mod tidy

    # Check if there are changes
    if git diff --quiet && git diff --cached --quiet; then
        log "No changes detected after update"
        git checkout main
        git branch -D "$branch_name"
        exit 0
    fi

    # Commit changes
    log "Committing changes..."
    git add -A
    git commit -m "chore: upgrade livetemplate to ${latest_version}"

    # Push branch
    log "Pushing branch..."
    git push -u origin "$branch_name"

    # Create PR
    log "Creating pull request..."
    local pr_url
    pr_url=$(gh pr create \
        --title "chore: upgrade livetemplate to ${latest_version}" \
        --body "$(cat <<EOF
## Summary
- Upgrades github.com/livetemplate/livetemplate from ${current_version} to ${latest_version}

## Changes
- Updated go.mod to use the latest livetemplate version
- Ran \`go mod tidy\`

## Test plan
- [ ] Verify all examples build successfully
- [ ] Run \`./test-all.sh\` to confirm all tests pass
EOF
)")

    log "Pull request created: $pr_url"
    log "Done!"
}

main "$@"
