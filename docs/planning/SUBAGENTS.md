# Junie Subagents Planning

This document outlines the roles, descriptions, and configurations for Junie subagents in the `groupscout` project. These subagents are designed to handle specific domains of the codebase with tailored instructions and tool sets.

## Subagent Roles

### 1. TDD Tester (`tdd-tester`)
- **Description:** Specializes in Test-Driven Development. Focuses on writing tests before implementation, ensuring high coverage, and verifying that code meets all requirements.
- **Tools:** `Read`, `Bash`, `Glob`, `Grep`, `Write`, `Edit`.
- **Skills:** `tdd-guidelines` (to be created).
- **Core Responsibilities:**
  - Create reproduction tests for bugs.
  - Draft test specifications for new features.
  - Refactor existing tests for better clarity and performance.
  - Ensure all tests pass before submitting changes.

### 2. Database Architect (`database-architect`)
- **Description:** Expert in database schema design, migrations, and performance optimization. Handles everything related to `internal/storage` and SQL migrations.
- **Tools:** `Read`, `Bash`, `Glob`, `Grep`, `Write`, `Edit`.
- **Skills:** `database-conventions`.
- **Core Responsibilities:**
  - Create and manage SQL migrations in the `migrations/` directory.
  - Optimize database queries and indexing strategies in `internal/storage/`.
  - Ensure data integrity and proper error handling in DB operations.
  - Handle database connection lifecycle and pooling configuration.

### 3. Collector Specialist (`collector-specialist`)
- **Description:** Specializes in data ingestion logic. Handles the implementation of new collectors in `internal/collector/` and ensures robust data fetching.
- **Tools:** `Read`, `Bash`, `Glob`, `Grep`, `Write`, `Edit`, `WebSearch`.
- **Skills:** `scraping-best-practices`.
- **Core Responsibilities:**
  - Implement new data collectors for external APIs or websites.
  - Handle rate limiting, retries, and error parsing for external sources.
  - Maintain existing collectors in `internal/collector/news`, `events`, and `permits`.
  - Ensure collectors adhere to the `internal/collector.Collector` interface.

### 4. Enrichment Processor (`enrichment-processor`)
- **Description:** Focuses on data transformation, enrichment, and AI-driven processing. Expert in `internal/enrichment` and `internal/ollama`.
- **Tools:** `Read`, `Bash`, `Glob`, `Grep`, `Write`, `Edit`.
- **Skills:** `ai-prompting-conventions`.
- **Core Responsibilities:**
  - Develop and refine data enrichment logic.
  - Manage LLM prompts and model files in `internal/ollama/modelfile`.
  - Implement scoring and extraction logic (e.g., lead scoring, disruption alerts).
  - Optimize the flow between raw data ingestion and enriched output.

### 5. API Integrator (`api-integrator`)
- **Description:** Expert in API design, middleware, and cross-service communication. Manages the interface between the core logic and external consumers.
- **Tools:** `Read`, `Bash`, `Glob`, `Grep`, `Write`, `Edit`.
- **Core Responsibilities:**
  - Implement and maintain API handlers in `internal/api/`.
  - Orchestrate logic between `server` and `alertd` daemons.
  - Manage notification logic in `internal/notify/` and `internal/alert/`.
  - Ensure robust request validation, error handling, and Slack command integration.

### 6. Infrastructure Specialist (`infrastructure-specialist`)
- **Description:** Focuses on containerization, build systems, and deployment automation. Expert in `Dockerfile`, `docker-compose.yml`, and `Makefile`.
- **Tools:** `Read`, `Bash`, `Glob`, `Grep`, `Write`, `Edit`, `WebSearch`.
- **Core Responsibilities:**
  - Maintain and optimize the Docker environment and `Makefile` commands.
  - Manage project configuration files and environment variable templates.
  - Automate build and deployment scripts in the `scripts/` directory.
  - Ensure system reliability and scalability across different deployment options.

## Implementation Details

The subagents will be implemented as Markdown files in the `.junie/agents/` directory.

### Directory Structure
```
.junie/
└── agents/
    ├── tdd-tester.md
    ├── database-architect.md
    ├── collector-specialist.md
    ├── enrichment-processor.md
    ├── api-integrator.md
    └── infrastructure-specialist.md
```

### Configuration Rules
- Subagents should use focused toolsets where possible.
- `collector-specialist` is the only one allowed to use `WebSearch` by default to find documentation for new APIs.
- All subagents must follow the project's coding standards as defined in `DEVELOPER.md` and `AGENTS.md`.
