# Building a Copilot Extension/MCP Server for ACR

## Goal

Create a way for Copilot users to invoke ACR's code review functionality from within Copilot Chat (e.g., `@acr review this PR`).

---

## Two Approaches

### Option A: Copilot Skillset (Hosted Service)

A **Copilot Extension** using the skillset approach where GitHub Copilot calls your hosted API endpoints.

**Architecture:**
```
User → Copilot Chat → GitHub → Your API Endpoint → ACR → Response
```

**Pros:**
- Works from anywhere (VS Code, GitHub.com, JetBrains)
- Centralized - one deployment serves all users
- Can access GitHub API for PR data

**Cons:**
- Requires hosting infrastructure (AWS, GCP, etc.)
- Must handle authentication, secrets management
- Need to clone repos to review them (security/cost implications)
- More complex deployment and maintenance

**Implementation:**
1. Create a GitHub App with Copilot Extension enabled
2. Build HTTP API endpoints that Copilot can call
3. Each skill receives parameters from Copilot and returns results
4. Host the service (e.g., on AWS Lambda, Cloud Run)

**Example skill definition:**
- Name: `review_pr`
- Description: "Review a GitHub PR for bugs and issues"
- Parameters: `{ "pr_url": "string" }`
- Endpoint: `https://your-api.com/review-pr`

---

### Option B: MCP Server (Local Tool) ⭐ Recommended

A **Model Context Protocol (MCP) server** that runs locally and integrates with Copilot Chat in VS Code.

**Architecture:**
```
User → VS Code Copilot Chat → MCP Client → ACR MCP Server → Local repo → Response
```

**Pros:**
- Runs locally - no hosting costs
- Direct access to local git repository
- Official Go SDK available (`github.com/modelcontextprotocol/go-sdk`)
- Simpler architecture - just wrap existing ACR functionality
- Privacy - code never leaves the machine
- Works with VS Code, JetBrains, Claude Desktop, and other MCP clients

**Cons:**
- User must install and configure locally
- Only works where MCP is supported (VS Code, JetBrains, etc.)
- Doesn't work from GitHub.com web interface

**Implementation:**
1. Create new `cmd/acr-mcp/main.go` - MCP server entry point
2. Expose ACR tools via MCP protocol:
   - `review_diff` - Review current changes
   - `review_pr` - Review a specific PR
   - `review_branch` - Review branch vs base
3. Use stdio transport (VS Code spawns server as subprocess)
4. User configures in VS Code's `mcp.json`

**Go SDK pattern:**
```go
import "github.com/modelcontextprotocol/go-sdk/mcp"

server := mcp.NewServer(&mcp.Implementation{Name: "acr", Version: "1.0.0"}, nil)
mcp.AddTool(server, &mcp.Tool{
    Name: "review_diff",
    Description: "Review code changes for bugs and issues",
}, reviewDiffHandler)
server.Run(context.Background(), &mcp.StdioTransport{})
```

**User configuration (VS Code `.vscode/mcp.json`):**
```json
{
  "servers": {
    "acr": {
      "command": "acr-mcp",
      "args": ["--agent", "claude"]
    }
  }
}
```

---

## Recommendation: MCP Server

**MCP is the better fit for ACR because:**

1. **Local-first** - ACR reviews local code; MCP runs locally
2. **Go SDK** - Official SDK matches ACR's language
3. **Simple integration** - Wrap existing runner/summarizer logic
4. **Growing ecosystem** - GitHub officially supports MCP in Copilot
5. **No infrastructure** - Users just install the binary

---

## Implementation Plan (MCP Server)

### Phase 1: Add MCP SDK Dependency

```bash
go get github.com/modelcontextprotocol/go-sdk@latest
```

### Phase 2: Create MCP Server Entry Point

**File: `cmd/acr-mcp/main.go`**

```go
package main

import (
    "context"
    "github.com/modelcontextprotocol/go-sdk/mcp"
    "github.com/richhaase/agentic-code-reviewer/internal/mcpserver"
)

func main() {
    server := mcp.NewServer(&mcp.Implementation{
        Name:    "acr",
        Version: "1.0.0",
    }, nil)

    mcpserver.RegisterTools(server)
    server.Run(context.Background(), &mcp.StdioTransport{})
}
```

### Phase 3: Create MCP Server Package

**File: `internal/mcpserver/tools.go`**

Define three MCP tools:

| Tool | Description | Parameters | Maps To |
|------|-------------|------------|---------|
| `review_changes` | Review changes vs base ref | `base_ref` (default: main), `agent` (default: codex) | `runner.Run()` |
| `review_pr` | Review a GitHub PR | `pr_number`, `agent` (default: codex) | Existing PR flow |
| `review_branch` | Review a branch vs base | `branch`, `base_ref`, `agent` | Worktree flow |

**Tool handler pattern:**
```go
func reviewChangesHandler(ctx context.Context, req *mcp.CallToolRequest, input ReviewChangesInput) (*mcp.CallToolResult, ReviewOutput, error) {
    // 1. Set up config from input params
    // 2. Create agents via agent.NewAgent()
    // 3. Run review via runner.New() + runner.Run()
    // 4. Aggregate via domain.AggregateFindings()
    // 5. Summarize via summarizer.Summarize()
    // 6. Format as markdown for Copilot
    return result, output, nil
}
```

### Phase 4: Extract Review Logic

Create a shared `internal/review/review.go` that encapsulates the review flow (currently in `cmd/acr/main.go:executeReview`):

```go
type ReviewRequest struct {
    WorkDir        string
    BaseRef        string
    AgentName      string
    SummarizerAgent string
    Reviewers      int
    Timeout        time.Duration
    // ... other config
}

type ReviewResult struct {
    Findings []domain.GroupedFinding
    Stats    domain.ReviewStats
    Markdown string  // Pre-formatted for MCP output
}

func Execute(ctx context.Context, req ReviewRequest) (*ReviewResult, error)
```

### Phase 5: Update Build System

**File: `Makefile`** - Add target:
```makefile
build-mcp:
	go build -o bin/acr-mcp ./cmd/acr-mcp
```

**File: `.goreleaser.yaml`** - Add binary:
```yaml
builds:
  - id: acr-mcp
    main: ./cmd/acr-mcp
    binary: acr-mcp
```

### Files to Create/Modify

| File | Action | Purpose |
|------|--------|---------|
| `cmd/acr-mcp/main.go` | Create | MCP server entry point |
| `internal/mcpserver/server.go` | Create | Server setup and registration |
| `internal/mcpserver/tools.go` | Create | Tool handlers |
| `internal/mcpserver/types.go` | Create | Input/output structs with JSON schema tags |
| `internal/review/review.go` | Create | Shared review execution logic |
| `cmd/acr/main.go` | Modify | Use shared review logic |
| `go.mod` | Modify | Add MCP SDK dependency |
| `Makefile` | Modify | Add build-mcp target |

### Verification Steps

1. **Build MCP server:**
   ```bash
   make build-mcp
   ```

2. **Test with Claude Desktop** (easiest MCP client):
   Add to `~/Library/Application Support/Claude/claude_desktop_config.json`:
   ```json
   {
     "mcpServers": {
       "acr": {
         "command": "/path/to/acr-mcp",
         "args": []
       }
     }
   }
   ```

3. **Test with VS Code Copilot Chat:**
   Add to `.vscode/mcp.json`:
   ```json
   {
     "servers": {
       "acr": {
         "command": "acr-mcp"
       }
     }
   }
   ```

4. **Verify tool invocation:**
   - In Copilot Chat: `@acr review the current changes`
   - Should return markdown-formatted findings

### Example Usage

After implementation, users can:

```
# In Copilot Chat (VS Code)
@acr review the current changes against main
@acr review PR #123
@acr review the feature-branch against develop
```

---

## Sources

- [About Copilot Skillsets](https://docs.github.com/en/copilot/building-copilot-extensions/building-a-copilot-skillset-for-your-copilot-extension/about-copilot-skillsets)
- [Building Copilot Skillsets](https://docs.github.com/en/copilot/building-copilot-extensions/building-a-copilot-skillset-for-your-copilot-extension/building-copilot-skillsets)
- [Extending Copilot Chat with MCP](https://docs.github.com/copilot/customizing-copilot/using-model-context-protocol/extending-copilot-chat-with-mcp)
- [MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk)
- [MCP Build Server Guide](https://modelcontextprotocol.io/docs/develop/build-server)
