#!/usr/bin/env bash
# =============================================================================
# session-start.sh — Superpowers bootstrap for Copilot CLI
# Adapted from: https://github.com/obra/superpowers/blob/main/hooks/session-start.sh
#
# WHAT THIS DOES:
# The original script injects context via hookSpecificOutput.additionalContext
# (Claude Code feature). Copilot CLI doesn't support context injection from
# hooks (GitHub CLI Issue #1139). This script writes to
# .github/copilot-instructions.md instead, which Copilot auto-reads.
#
# PATHS: Supports OpenCode convention (.agents/skills/) as primary,
# with fallback to .github/skills/ and .claude/skills/
# =============================================================================

set -euo pipefail

# ---------------------------------------------------------------------------
# Resolve project root (assume script is at <project>/scripts/session-start.sh)
# ---------------------------------------------------------------------------
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

INSTRUCTIONS_FILE="${PROJECT_ROOT}/session-instructions.md"
MARKER="<!-- SUPERPOWERS-MANAGED -->"
CACHE_FILE="${PROJECT_ROOT}/.github/.superpowers-hash"

# ---------------------------------------------------------------------------
# Idempotency: Copilot sessionStart fires on EVERY prompt (Issue #991).
# Skip regeneration if skills/plans/agents haven't changed.
# ---------------------------------------------------------------------------
compute_fingerprint() {
    # Hash: skill file paths + modification dates
    # Uses 'date -r' which works across Git Bash, macOS, and Linux
    local output=""
    for d in "${PROJECT_ROOT}/.agents/skills" \
             "${PROJECT_ROOT}/.github/skills" \
             "${PROJECT_ROOT}/.claude/skills"; do
        if [ -d "$d" ]; then
            while IFS= read -r f; do
                output+="${f} $(date -r "$f" +%s 2>/dev/null || echo 0)"$'\n'
            done < <(find "$d" -name "SKILL.md" 2>/dev/null)
        fi
    done
    if [ -d "${PROJECT_ROOT}/docs/plans" ]; then
        while IFS= read -r f; do
            output+="${f} $(date -r "$f" +%s 2>/dev/null || echo 0)"$'\n'
        done < <(find "${PROJECT_ROOT}/docs/plans" -name "*.md" 2>/dev/null)
    fi
    while IFS= read -r f; do
        [ -n "$f" ] && output+="${f}"$'\n'
    done < <(find "${PROJECT_ROOT}" -maxdepth 3 -name "*.agent.md" -type f 2>/dev/null)

    if [ -z "$output" ]; then
        echo "no-cache"
    else
        echo "$output" | md5sum | cut -d' ' -f1
    fi
}

current_hash=$(compute_fingerprint)
if [ -f "$CACHE_FILE" ] && [ -f "$INSTRUCTIONS_FILE" ]; then
    cached_hash=$(cat "$CACHE_FILE" 2>/dev/null || echo "")
    if [ "$current_hash" = "$cached_hash" ]; then
        echo "[superpowers] No changes detected, skipping regeneration" >&2
        exit 0
    fi
fi

# ---------------------------------------------------------------------------
# Discover skills directories (priority order: .agents > .github > .claude)
# ---------------------------------------------------------------------------
SKILLS_DIRS=()
for candidate in \
    "${PROJECT_ROOT}/.agents/skills" \
    "${PROJECT_ROOT}/.github/skills" \
    "${PROJECT_ROOT}/.claude/skills"; do
    if [ -d "$candidate" ]; then
        SKILLS_DIRS+=("$candidate")
    fi
done

# Also check global/user-level skills
for candidate in \
    "${HOME}/.config/opencode/superpowers/skills" \
    "${HOME}/.config/opencode/skills" \
    "${HOME}/.copilot/skills" \
    "${HOME}/.claude/skills"; do
    if [ -d "$candidate" ]; then
        SKILLS_DIRS+=("$candidate")
    fi
done

if [ ${#SKILLS_DIRS[@]} -eq 0 ]; then
    echo "[superpowers] WARNING: No skills directories found" >&2
    echo "[superpowers] Expected one of: .agents/skills/, .github/skills/, .claude/skills/" >&2
    exit 0
fi

echo "[superpowers] Found skills in: ${SKILLS_DIRS[*]}" >&2

# ---------------------------------------------------------------------------
# Build skills catalog (deduplicate by skill name, first-found wins)
# ---------------------------------------------------------------------------
declare -A SEEN_SKILLS
SKILLS_TABLE=""
SKILL_COUNT=0

for skills_dir in "${SKILLS_DIRS[@]}"; do
    # Make path relative to project root if possible
    rel_dir="${skills_dir#${PROJECT_ROOT}/}"
    # If it didn't change, it's an absolute path (global skill)
    if [ "$rel_dir" = "$skills_dir" ]; then
        rel_dir="$skills_dir"  # Keep absolute for global
    fi

    for skill_dir in "$skills_dir"/*/; do
        [ -d "$skill_dir" ] || continue
        skill_name="$(basename "$skill_dir")"

        # Deduplicate: project skills override global
        if [ -n "${SEEN_SKILLS[$skill_name]+x}" ]; then
            continue
        fi
        SEEN_SKILLS[$skill_name]=1

        skill_file="${skill_dir}SKILL.md"
        if [ ! -f "$skill_file" ]; then
            continue
        fi

        # Extract description from YAML frontmatter
        description=""
        if head -1 "$skill_file" | grep -q '^---'; then
            description=$(sed -n '/^---$/,/^---$/p' "$skill_file" \
                | grep -i '^description:' \
                | head -1 \
                | sed 's/^[Dd]escription:[[:space:]]*//')
        fi
        if [ -z "$description" ]; then
            description="(see SKILL.md)"
        fi

        rel_skill="${skill_dir#${PROJECT_ROOT}/}"
        if [ "$rel_skill" = "$skill_dir" ]; then
            rel_skill="$skill_dir"
        fi

        SKILLS_TABLE="${SKILLS_TABLE}| ${skill_name} | ${description} | \`${rel_skill}SKILL.md\` |
"
        SKILL_COUNT=$((SKILL_COUNT + 1))
    done
done

echo "[superpowers] Cataloged ${SKILL_COUNT} skills" >&2

# ---------------------------------------------------------------------------
# Read using-superpowers SKILL.md (the core bootstrap skill)
# ---------------------------------------------------------------------------
USING_SUPERPOWERS=""
for skills_dir in "${SKILLS_DIRS[@]}"; do
    candidate="${skills_dir}/using-superpowers/SKILL.md"
    if [ -f "$candidate" ]; then
        USING_SUPERPOWERS=$(cat "$candidate")
        echo "[superpowers] Loaded using-superpowers from: ${candidate}" >&2
        break
    fi
done

# ---------------------------------------------------------------------------
# Discover active plans/designs in docs/plans/
# ---------------------------------------------------------------------------
PLANS_SECTION=""
if [ -d "${PROJECT_ROOT}/docs/plans" ]; then
    plan_files=$(find "${PROJECT_ROOT}/docs/plans" -name "*.md" -type f 2>/dev/null | sort -r)
    if [ -n "$plan_files" ]; then
        PLANS_SECTION="## Active Plans & Designs

Before implementing any feature, check if there's an existing plan:

| Document | Path |
|----------|------|
"
        while IFS= read -r plan; do
            plan_rel="${plan#${PROJECT_ROOT}/}"
            plan_name="$(basename "$plan" .md)"
            PLANS_SECTION="${PLANS_SECTION}| ${plan_name} | \`${plan_rel}\` |
"
        done <<< "$plan_files"

        PLANS_SECTION="${PLANS_SECTION}
**Read the relevant plan before starting work.** Use \`cat <path>\` to load it.
"
        echo "[superpowers] Found $(echo "$plan_files" | wc -l | tr -d ' ') plan documents" >&2
    fi
fi

# ---------------------------------------------------------------------------
# Discover agent files
# ---------------------------------------------------------------------------
AGENTS_SECTION=""
agent_files=""
for agent_dir in \
    "${PROJECT_ROOT}/.agents" \
    "${PROJECT_ROOT}/.github/agents" \
    "${PROJECT_ROOT}/.copilot/agents"; do
    if [ -d "$agent_dir" ]; then
        found=$(find "$agent_dir" -maxdepth 1 -name "*.agent.md" -type f 2>/dev/null)
        if [ -n "$found" ]; then
            agent_files="${agent_files}${found}
"
        fi
    fi
done

if [ -n "$agent_files" ]; then
    AGENTS_SECTION="## Available Agents

| Agent | Path |
|-------|------|
"
    while IFS= read -r agent; do
        [ -z "$agent" ] && continue
        agent_rel="${agent#${PROJECT_ROOT}/}"
        agent_name="$(basename "$agent" .agent.md)"
        AGENTS_SECTION="${AGENTS_SECTION}| ${agent_name} | \`${agent_rel}\` |
"
    done <<< "$agent_files"
    echo "[superpowers] Found agent files" >&2
fi

# ---------------------------------------------------------------------------
# Generate .github/copilot-instructions.md
# ---------------------------------------------------------------------------
mkdir -p "$(dirname "$INSTRUCTIONS_FILE")"

cat > "$INSTRUCTIONS_FILE" << 'HEADER'
<!-- SUPERPOWERS-MANAGED -->
<!-- ⚠️  AUTO-GENERATED by scripts/session-start.sh — Do not edit manually -->
<!-- Regenerate: ./scripts/session-start.sh -->

HEADER

# --- THE RULE (core superpowers principle) ---
cat >> "$INSTRUCTIONS_FILE" << 'THERULE'
## The Rule

**Invoke relevant skills BEFORE any response or action.** Even if there's only a 1% chance a skill applies, read it first.

### How to Load a Skill

You do NOT have a "Skill tool." Instead, **read the skill file directly**:

```
cat .agents/skills/<skill-name>/SKILL.md
```

Then follow the instructions in that file completely.

### Red Flags (excuses you must NOT make)

- ❌ "This is a simple task" — Simple tasks done wrong waste the most time
- ❌ "I already know how to do this" — The skill may have project-specific rules
- ❌ "It would slow me down" — It takes 2 seconds to read a file
- ❌ "The user didn't ask me to" — THE RULE says invoke skills proactively

### Skill Priority

1. **Project skills** (`.agents/skills/`) — highest priority
2. **User skills** (`~/.config/opencode/skills/`) — personal overrides
3. **Superpowers skills** (global install) — defaults

THERULE

# --- Using-superpowers content (if found) ---
if [ -n "$USING_SUPERPOWERS" ]; then
    cat >> "$INSTRUCTIONS_FILE" << USINGSKILLS
## Superpowers System Reference

<details>
<summary>Full using-superpowers skill (click to expand)</summary>

${USING_SUPERPOWERS}

</details>

USINGSKILLS
fi

# --- Skills catalog ---
cat >> "$INSTRUCTIONS_FILE" << CATALOG
## Skills Catalog

${SKILL_COUNT} skills available. **Read the SKILL.md before starting related work.**

| Skill | Description | Path |
|-------|-------------|------|
${SKILLS_TABLE}
To load a skill: \`cat <path>\` — then follow its instructions.

CATALOG

# --- Plans section ---
if [ -n "$PLANS_SECTION" ]; then
    echo "$PLANS_SECTION" >> "$INSTRUCTIONS_FILE"
fi

# --- Agents section ---
if [ -n "$AGENTS_SECTION" ]; then
    echo "$AGENTS_SECTION" >> "$INSTRUCTIONS_FILE"
fi

# --- Tool mapping (Claude Code → Copilot) ---
cat >> "$INSTRUCTIONS_FILE" << 'TOOLMAP'
## Tool Mapping (Superpowers → Copilot)

Skills were written for Claude Code. Use these Copilot equivalents:

| Skill References | Copilot Equivalent |
|-----------------|-------------------|
| "Use the Skill tool" | `cat .agents/skills/<name>/SKILL.md` |
| `TodoWrite` / `TodoRead` | Use a `TODO.md` file or GitHub Issues |
| `Task` (subagent) | Not available — do the work inline, or note it for the user |
| `Bash("git ...")` | `git ...` (direct shell) |
| "Claude Code" | "Copilot CLI" (you are running in Copilot) |

TOOLMAP

echo "[superpowers] Generated ${INSTRUCTIONS_FILE}" >&2
echo "[superpowers] Done — ${SKILL_COUNT} skills, ready for Copilot" >&2

# Save hash for idempotency
echo "$current_hash" > "$CACHE_FILE"
