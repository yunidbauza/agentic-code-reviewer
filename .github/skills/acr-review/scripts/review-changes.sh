#!/bin/bash
set -e

BASE_REF="main"
REVIEWERS=5
AGENT="copilot"

while [[ $# -gt 0 ]]; do
    case $1 in
        --base-ref) BASE_REF="$2"; shift 2 ;;
        --reviewers) REVIEWERS="$2"; shift 2 ;;
        --agent) AGENT="$2"; shift 2 ;;
        *) echo "Unknown option: $1"; exit 1 ;;
    esac
done

ACR_BIN="$HOME/.acr/acr"

if [ ! -f "$ACR_BIN" ]; then
    echo "Error: ACR not found at $ACR_BIN"
    echo ""
    echo "Run the setup script first to build ACR:"
    echo "./.github/skills/acr-review/scripts/setup-acr.sh"
    exit 1
fi

"$ACR_BIN" --base "$BASE_REF" --reviewers "$REVIEWERS" --agent "$AGENT"
