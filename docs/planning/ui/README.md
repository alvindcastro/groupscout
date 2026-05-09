# UI Planning Docs

This folder groups planning documents for the future GroupScout operator UI.

## Documents

- [UI_STRATEGY.md](./UI_STRATEGY.md) - Product flow, information architecture, screen map, API boundaries, and MVP sequence.
- [UI_DESIGN_SYSTEM.md](./UI_DESIGN_SYSTEM.md) - GroupScout adaptation of the external design reference for operator UI surfaces.
- [UI_PHASE31_DESIGN_SYSTEM_CONTRACT.md](./UI_PHASE31_DESIGN_SYSTEM_CONTRACT.md) - Phase 31 token, primitive, component test, accessibility, secret-safety, and no-hero implementation contract.
- [UI_PHASE32_APP_SHELL_ROUTING_CONTRACT.md](./UI_PHASE32_APP_SHELL_ROUTING_CONTRACT.md) - Phase 32 app shell, route, responsive layout, build, and component-test implementation contract.
- [UI_PHASE33_MOCKED_LEAD_INBOX_CONTRACT.md](./UI_PHASE33_MOCKED_LEAD_INBOX_CONTRACT.md) - Phase 33 mocked lead inbox fixture, table, interaction, accessibility, responsive, and replaceable API-boundary implementation contract.
- [UI_PHASE34_LEAD_DETAIL_EVIDENCE_CONTRACT.md](./UI_PHASE34_LEAD_DETAIL_EVIDENCE_CONTRACT.md) - Phase 34 lead detail, evidence review, raw audit safety, outreach/activity, status action, and correction-control implementation contract.
- [UI_PHASE35_API_CONTRACT.md](./UI_PHASE35_API_CONTRACT.md) - Phase 35 implemented `/api/leads` list, detail, patch, raw evidence, auth, and schema-gap contract.
- [UI_PHASE35_FRONTEND_TYPES.md](./UI_PHASE35_FRONTEND_TYPES.md) - Temporary TypeScript type contract for the Phase 35 API until generated frontend client types exist.
- [UI_PHASE36_OUTREACH_STATE_CONTRACT.md](./UI_PHASE36_OUTREACH_STATE_CONTRACT.md) - Phase 36 outreach log, lead action, transition validation, and workflow-field contract.
- [UI_PHASE37_PIPELINE_STATS_SYSTEM_CONTRACT.md](./UI_PHASE37_PIPELINE_STATS_SYSTEM_CONTRACT.md) - Phase 37 pipeline run, stats, and system-health API contract.
- [UI_PHASE38_DOCKER_SMOKE_CONTRACT.md](./UI_PHASE38_DOCKER_SMOKE_CONTRACT.md) - Phase 38 backend plus external UI Docker smoke contract.
- [UI_API_ENDPOINTS.md](./UI_API_ENDPOINTS.md) - Brainstormed `/api/*` contracts for the future operator UI.
- [UI_TDD_PHASE_PLAN.md](./UI_TDD_PHASE_PLAN.md) - Phase-by-phase strict TDD plan for UI and backend contract work.
- [BACKEND_FOR_UI_TESTING.md](./BACKEND_FOR_UI_TESTING.md) - Runbook for starting the backend while developing or testing the UI.
- [BACKEND_FRONTEND_DOCKER_E2E.md](./BACKEND_FRONTEND_DOCKER_E2E.md) - Smoke runbook for running the backend Compose stack with the separate UI Docker image.
- [PROMPTS_PHASE32_UI.md](../../prompts/PROMPTS_PHASE32_UI.md) - Copy-paste strict TDD prompts for Phase 32 route, shell, responsive, build, and component-test work.
- [PROMPTS_PHASE33_UI.md](../../prompts/PROMPTS_PHASE33_UI.md) - Copy-paste strict TDD prompts for Phase 33 mocked lead inbox fixture, table, search, filter, sorting, keyboard, responsive, and build verification work.
- [PROMPTS_PHASE34_UI.md](../../prompts/PROMPTS_PHASE34_UI.md) - Copy-paste strict TDD prompts for Phase 34 lead detail, evidence, raw audit safety, outreach/activity, status action, correction-control, responsive, and build verification work.
- [PROMPTS_PHASE35_UI.md](../../prompts/PROMPTS_PHASE35_UI.md) - Copy-paste strict TDD prompts for Phase 35 UI API contract refinement, docs, and type maintenance.

## Rule

UI-specific Markdown should either live in this folder or include `UI` in the filename.
