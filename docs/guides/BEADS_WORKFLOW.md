# Beads Workflow for Markdown Prompt Users

This guide explains how to use `bd` Beads for task tracking while keeping Markdown files useful for prompts, plans, specs, and handoffs.

## Core Rule

Use Beads for live work state. Use Markdown for reusable context.

Do not track active work with Markdown TODO lists in this repository. A Markdown file can describe a phase, prompt, or design, but the active unit of work should be a bead with status, priority, acceptance criteria, dependencies, and completion history.

## Mental Model

If you are used to writing prompts like this:

```text
Implement P11 in docs/phases-and-tasks.md.
Refer to necessary MD files.
Run parallel Codex agents if useful.
Update docs and CHANGELOG.md.
Do not commit, but provide a commit message.
```

Convert it into this Beads-backed flow:

1. Read the Markdown prompt or phase document for context.
2. Create or claim a bead for the actual work.
3. Put acceptance criteria and design constraints on the bead.
4. Use linked beads for independent subtasks or discovered follow-up work.
5. Update Markdown docs when behavior, design, or usage changes.
6. Update `docs/CHANGELOG.md` with what, where, when, why, and how.
7. Close the bead when the work is verified.
8. Commit with a message that describes the outcome.

## Prompt Section Mapping

| Markdown prompt section | Beads location |
|---|---|
| Goal | Bead title and description |
| Context | `--description` or `--notes` |
| Constraints | `--design` |
| Done when | `--acceptance` |
| Steps | Child beads or implementation notes |
| Follow-ups | New beads linked with `discovered-from` |
| Status | Bead status, not Markdown checkboxes |

## Essential Commands

Run this at the start of a session:

```bash
bd prime
bd dolt pull
bd ready --json
```

Inspect available work:

```bash
bd ready --json
bd show <id> --json
bd list --status=open --json
bd list --status=in_progress --json
```

Create a task from a Markdown prompt:

```bash
bd create \
  --title="Implement P11 from docs/phases-and-tasks.md" \
  --description="Implement P11 using docs/phases-and-tasks.md and referenced Markdown context." \
  --acceptance="P11 is implemented, relevant docs are updated, docs/CHANGELOG.md records what/where/when/why/how, and verification is run." \
  --design="Use existing repository patterns. Keep Markdown for documentation and Beads for active task tracking." \
  --type=task \
  --priority=1 \
  --json
```

Claim work before changing files:

```bash
bd update <id> --claim --json
```

Create discovered follow-up work:

```bash
bd create \
  --title="Fix issue found while implementing P11" \
  --description="Describe the bug, risk, or follow-up clearly." \
  --type=bug \
  --priority=2 \
  --deps discovered-from:<parent-id> \
  --json
```

Add a blocking dependency:

```bash
bd dep add <issue-id> <depends-on-id>
bd blocked --json
```

Close completed work:

```bash
bd close <id> --reason="Implemented, documented, changelog updated, and verified." --json
```

Sync Beads metadata:

```bash
bd dolt pull
bd dolt push
```

Run hygiene checks:

```bash
bd lint
bd doctor
bd preflight
bd stale
bd orphans
```

## Parallel Codex Agents

Use parallel agents only when the work can be split cleanly.

Good parallel work:

- One agent researches the phase docs.
- One agent inspects related code paths.
- One agent drafts documentation updates.
- One agent implements a bounded module with a clear file ownership area.

Avoid parallel agents when:

- The next step depends on one blocking answer.
- Multiple agents would edit the same files.
- The task is small enough that coordination costs more than the work.

The main agent should own integration, final verification, Beads updates, and the final commit.

## Markdown Files Still Matter

Markdown is still the right place for:

- Phase descriptions and product plans.
- Prompt libraries and reusable instruction templates.
- Architecture notes.
- Setup, testing, and troubleshooting guides.
- Changelog entries.
- Human-readable handoff notes.

Markdown is not the right place for:

- Active TODO lists.
- Current task status.
- Dependency tracking.
- Ownership tracking.
- Completion state.

Use `bd` for those.

## Changelog Entry Format

Every completed task should update `docs/CHANGELOG.md` with:

- What changed.
- Where it changed.
- When it changed.
- Why it changed.
- How it was implemented.

Example:

```md
## 2026-05-08 - Beads prompt workflow guide

### groupscout-3jw - Document Beads prompt workflow

- **What:** Added a guide for converting Markdown prompt habits into Beads-backed task tracking.
- **Where:** `docs/guides/BEADS_WORKFLOW.md`.
- **When:** 2026-05-08.
- **Why:** Prompt-heavy workflows need a clear split between reusable Markdown context and live Beads task state.
- **How:** Documented command flows, prompt-to-bead mapping, parallel agent guidance, changelog expectations, and commit-message practices.
```

## Commit Flow

For normal completed work, commit the change. Use "do not commit" only for explicit draft sessions.

Recommended completion flow:

```bash
git status
bd close <id> --reason="Implemented, documented, changelog updated, and verified." --json
bd dolt pull
bd dolt push
git add <changed-files>
git commit -m "Document Beads prompt workflow"
git status
```

If the repository instructions require a push at session end, finish with:

```bash
git pull --rebase
git push
git status
```

## Commit Message Pattern

Use a concise subject that names the outcome:

```text
Document Beads prompt workflow
```

Use a body when multiple areas changed:

```text
Document Beads prompt workflow

- Add a guide for moving Markdown prompt tasks into Beads
- Document core bd commands and prompt-to-bead mapping
- Record changelog expectations and completion flow
```

Avoid commit messages that only describe process:

```text
Update files
Run agent
Make changes
```

## Reusable Prompt Template

Use this when asking an agent to work from a Markdown phase or prompt file:

```text
Use Beads for task tracking.

Goal:
Implement <phase-or-task-id> from <markdown-file>.

Before coding:
1. Run bd prime.
2. Inspect the referenced Markdown file and any linked docs needed for context.
3. Create or claim a Beads issue with --json.
4. Add acceptance criteria and design notes to the bead.
5. If the work splits cleanly, create linked beads for independent subtasks.

Execution:
- Use parallel Codex agents only for independent, non-overlapping research or implementation tasks.
- Keep the main agent responsible for integration and final verification.
- Update necessary Markdown docs after behavior, design, or usage changes.
- Do not use Markdown TODO lists for task tracking.

Required docs updates:
- Update or create relevant Markdown docs.
- Update docs/CHANGELOG.md with what, where, when, why, and how.

Verification:
- Run relevant tests, linters, builds, or documentation checks.
- Record skipped verification and why.

Completion:
- Close completed Beads issues with a clear reason.
- Commit the completed work unless this is explicitly a draft-only session.
- Provide the commit hash, commit message, files changed, and verification results.
```
