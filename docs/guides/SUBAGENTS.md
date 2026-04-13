# Junie Subagents Guide

This guide explains how to use the specialized Junie subagents in the `groupscout` project. These subagents are designed to handle specific domains of the codebase with tailored instructions and tool sets.

## 🤖 Available Subagents

We have defined the following subagents in `.junie/agents/`. Each subagent has specific core responsibilities and a tailored toolset.

### 1. TDD Tester (`tdd-tester`)
- **Focus:** Test-Driven Development and high coverage.
- **Area:** `*_test.go`, `internal/`
- **Responsibilities:**
  - Create reproduction tests for bugs.
  - Draft test specifications for new features.
  - Refactor existing tests for better clarity and performance.
  - Ensure all tests pass before submitting changes.

### 2. Database Architect (`database-architect`)
- **Focus:** SQL migrations and storage layer optimization.
- **Area:** `migrations/`, `internal/storage/`
- **Responsibilities:**
  - Create and manage SQL migrations in the `migrations/` directory.
  - Optimize database queries and indexing strategies in `internal/storage/`.
  - Ensure data integrity and proper error handling in DB operations.
  - Handle database connection lifecycle and pooling configuration.

### 3. Collector Specialist (`collector-specialist`)
- **Focus:** Data ingestion logic and scraping.
- **Area:** `internal/collector/`
- **Responsibilities:**
  - Implement new data collectors for external APIs or websites.
  - Handle rate limiting, retries, and error parsing for external sources.
  - Maintain existing collectors in `internal/collector/news`, `events`, and `permits`.
  - Ensure collectors adhere to the `internal/collector.Collector` interface.

### 4. Enrichment Processor (`enrichment-processor`)
- **Focus:** Data transformation and AI processing.
- **Area:** `internal/enrichment/`, `internal/ollama/`
- **Responsibilities:**
  - Develop and refine data enrichment logic.
  - Manage LLM prompts and model files in `internal/ollama/modelfile`.
  - Implement scoring and extraction logic (e.g., lead scoring, disruption alerts).
  - Optimize the flow between raw data ingestion and enriched output.

### 5. API Integrator (`api-integrator`)
- **Focus:** API design, middleware, and service integration.
- **Area:** `internal/api/`, `internal/alert/`, `cmd/`
- **Responsibilities:**
  - Implement and maintain API handlers in `internal/api/`.
  - Orchestrate logic between `server` and `alertd` daemons.
  - Manage notification logic in `internal/notify/` and `internal/alert/`.
  - Ensure robust request validation, error handling, and Slack command integration.

### 6. Infrastructure Specialist (`infrastructure-specialist`)
- **Focus:** Docker, Makefile, and deployment.
- **Area:** `Dockerfile`, `Makefile`, `scripts/`
- **Responsibilities:**
  - Maintain and optimize the Docker environment and `Makefile` commands.
  - Manage project configuration files and environment variable templates.
  - Automate build and deployment scripts in the `scripts/` directory.
  - Ensure system reliability and scalability across different deployment options.

## 🚀 How to Use

When using the Junie CLI, you can invoke a specific subagent using the `--agent` (or `-a`) flag.

### 1. General Usage
To start a session with a specific subagent:
```bash
junie --agent tdd-tester "Implement a reproduction test for the issue in internal/storage/lead.go"
```

### 2. Delegating Tasks
If you are already in a Junie session (using the default agent), you can ask Junie to use a subagent for a specific task. Junie will switch its persona and toolset according to the subagent's definition.

### 3. Combining Subagents (Choreography)
For complex tasks, you might use different subagents sequentially. This is called "Agent Choreography." For example, to implement the Audit Trail feature (Phase 27), you would:
1. Use `database-architect` to create a new migration and storage layer.
2. Use `collector-specialist` to update scrapers to capture raw data.
3. Use `enrichment-processor` to link raw input to the generated leads.
4. Use `api-integrator` to implement the API/CLI for manual verification.
5. Use `tdd-tester` at each step to ensure everything is properly tested.

See [AGENT_CHOREOGRAPHY_PHASE27.md](../planning/AGENT_CHOREOGRAPHY_PHASE27.md) for a detailed execution plan of this workflow.

#### 💡 Best Practices for Choreography:
- **Small Contexts:** Only give each agent the information relevant to its specific task.
- **Explicit Hand-offs:** Define clear deliverables for each agent (e.g., `database-architect` delivers a schema, `api-integrator` consumes it).
- **Iterative Testing:** Use `tdd-tester` to verify each component in isolation before moving to the next stage of the choreography.
- **Central Planning:** The main Junie agent (you) should maintain the high-level plan while subagents handle the domain-specific implementation details.

## 🛠 Adding or Updating Subagents

Subagent definitions are stored in `.junie/agents/` as Markdown files with a YAML frontmatter.

### Subagent File Format:
```yaml
---
name: "agent-name"
description: "Brief description of the agent's role."
tools: ["Read", "Bash", "Glob", "Grep", "Write", "Edit", "WebSearch", "AskUserQuestion", "update_status"]
---
# Agent Name

Detailed instructions for the agent...
```

### Tips for Better Subagents:
- **Platform Tools:** Always include `update_status` and `AskUserQuestion` in the `tools` list if you use a restricted allowlist. This ensures Junie can still report progress and ask for clarification.
- **Focused Tools:** Only give an agent the tools it needs (e.g., `WebSearch` for `collector-specialist`).
- **Clear Instructions:** Use the instructions to define the agent's persona, core principles, and common tasks.
- **Reference Docs:** Point the agent to relevant project documentation (e.g., `DEVELOPER.md`, `PHASES.md`).

## 🔗 Related Resources
- [Junie CLI Documentation](https://junie.jetbrains.com/docs/junie-cli-subagents.html)
- [DEVELOPER.md](../DEVELOPER.md)
