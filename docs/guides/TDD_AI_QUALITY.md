# AI Quality TDD Policy — groupscout

This policy applies to every code task in the AI quality/eval/monitoring extension.

## Non-negotiable loop

Every code task follows this sequence:

```text
1. Read the relevant docs, tests, and existing package patterns.
2. Add or update the smallest failing test that proves the missing behavior.
3. Run the narrow test and confirm it fails for the expected reason.
4. Implement only enough production code to pass.
5. Run the narrow test again and confirm green.
6. Run the broader suite: `go test ./...`.
7. Refactor only while tests stay green.
8. Update Markdown/docs/report examples only when behavior, commands, thresholds, or acceptance criteria changed.
```

## Forbidden shortcuts

- Do not write production Go code before the failing test exists.
- Do not call live LLMs, live cloud search, Slack, Resend, Sentry, Botpress, external threat feeds, or public websites from unit tests.
- Do not add secrets, real customer data, real student data, real incident evidence, or private portal content to fixtures.
- Do not mark a task done because the code compiles; task completion requires acceptance evidence.
- Do not allow critical safety, privacy, unsupported-claim, or unauthorized-action checks to fail open.

## Required coverage for code tasks

Every code task must include tests for the applicable items:

- happy path,
- invalid input,
- timeout or dependency failure,
- redaction/privacy boundary,
- no-result or low-confidence behavior,
- action/tool gating,
- report/gate output if the task affects eval results,
- observability attributes if the task emits traces, logs, or metrics.

## Evidence to leave in the task summary

When finishing a code task, record:

```text
Files changed:
Tests added/updated:
Red evidence:
Green evidence:
Broader command:
Behavior added:
Known residual risk:
Follow-up case to add later:
```

## Preferred Go test tools

- `testing` and table-driven tests first.
- `httptest.Server` for HTTP clients and adapters.
- Fake interfaces for LLM, search, tools, workflows, collectors, and judges.
- Golden files for stable Markdown, JSON, XML, or prompt outputs.
- `context.WithTimeout` tests for external-call boundaries.
- `t.TempDir()` for report generation.

## Definition of done for eval/release-gate tasks

A code task is done only when:

- the new failing test was observed red before implementation,
- the narrow test passes,
- `go test ./...` passes or the failure is documented as unrelated and reproducible,
- generated reports are deterministic,
- critical gates fail closed,
- docs and task boards are updated.
