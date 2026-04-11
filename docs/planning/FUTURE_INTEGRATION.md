# Future Integration Roadmap

> Maps brainstorm ideas to concrete GroupScout phases and PHASES.md tasks.
> Last updated: 2026-04-10

---

## 1. Agentic Engineering & GenAI Workflows

*Target: `internal/enrichment`, `internal/tools`*
*Planned phase: **Phase 23 — Agentic Reasoning & Tool-Calling***

| Idea | Status | Phase |
|---|---|---|
| Reasoning Loops (ReAct/Plan-and-Solve) for complex permits | 📋 planned | Phase 23-A |
| RAG: top-k similar past leads injected into prompt context | ✅ done (pgvector, Phase 15) | — |
| Tool-Calling: BC Registry lookup + LinkedIn URL generation | 📋 planned | Phase 23-B |
| Multimodal PDF parsing via Claude vision | ⏸ deferred (5+ cities threshold) | Phase 23-C |
| Multi-agent pipeline: one agent per source, coordinator deduplicates | 📋 future | Phase 26+ |

**Next concrete task:** `internal/enrichment/react_test.go` — write `TestReactEnricher_CallsToolOnBorderlineScore` with mock LLM; fail first.

---

## 2. Data Foundation & AI Pipelines

*Target: `internal/collector`, `internal/storage`, `internal/enrichment`*
*Planned phase: **Phase 24 — AI Observability & Quality***

| Idea | Status | Phase |
|---|---|---|
| AI-Ready SQL (`v_lead_context` view + `GetContext()`) | 📋 planned | Phase 24-A |
| Prompt refactor to use `GetContext()` instead of hand-built strings | 📋 planned | Phase 24-A |
| Langfuse LLM observability (token cost, latency, prompt version) | 📋 planned | Phase 24-B |
| Enrichment eval harness (`EvalLead()` structural validation) | 📋 planned | Phase 24-C |
| AI observability eval (RAGAS / Vertex Eval for hallucination detection) | 📋 future | Phase 26+ |
| Unstructured PDF ingestion (AI-driven parsing for tender docs) | 📋 future | Phase 23-C |

**Next concrete task:** `migrations/005_ai_context.up.sql` — `v_lead_context` view; then `TestGetContext_ReturnsExpectedString`.

---

## 3. Integration & Cloud-Native Development

*Target: `config`, `terraform/`, `cmd/server/`*
*Planned phase: **Phase 25 — Cloud-Native & Event-Driven***

| Idea | Status | Phase |
|---|---|---|
| LLM Provider Abstraction (`LLMClient` interface, factory) | 📋 planned | Phase 16 |
| Hetzner CX32 + Coolify production deploy (primary recommendation, ~$10/month) | 📋 planned | Phase 25-A |
| Event-driven ingestion: `POST /ingest` → `EnrichOne()` (platform-agnostic) | 📋 planned | Phase 25-B |
| Terraform IaC for GCP (optional; alertd incompatible with Cloud Run — needs Compute Engine VM) | 📋 optional | Phase 25-C |
| AIaaS API Layer: expose enrichment as standalone service | 📋 future | Phase 10 |
| CRM/ERP Integration: HubSpot, Salesforce, SAP | 📋 future | Phase 11 |

**Next concrete task (Phase 25-A):** Provision Hetzner CX32, install Coolify, deploy from existing `docker-compose.yml`, verify `/health` → 200. Write `docs/guides/COOLIFY.md`.

---

## 4. Agile Execution & Quality

*Target: `docs/guides/TESTING.md`, `internal/enrichment/eval.go`*

| Idea | Status | Phase |
|---|---|---|
| TDD-first rule enforced per phase (T tasks before impl tasks) | ✅ documented | All phases 16+ |
| Enrichment structural eval (`EvalLead()` + Sentry on hard failures) | 📋 planned | Phase 24-C |
| UAT suites validating AI-generated endpoints + outreach drafts | 📋 planned | Phase 24-C |
| Benchmark test for pipeline throughput (target: <30s for 20 permits) | 📋 future | Phase 9 |

---

## Summary of Next Steps

By priority:

1. **Phase 7.4–7.6** — Lead Management API endpoints (`GET /leads`, `PATCH /leads/{id}`, OutreachLogStore) — unblocks Phase 19
2. **Phase 16** — LLM Provider Abstraction — required before Phase 23 (agentic layer depends on the interface)
3. **Phase 24-A** — AI-Ready SQL view — quick win; immediately improves prompt quality and reduces `claude.go` complexity
4. **Phase 9** — Parallel collector execution — reduces pipeline runtime from sequential to concurrent
5. **Phase 23** — ReAct loop + tool-calling — highest AI value; requires Phase 16 first
6. **Phase 25** — Terraform IaC — enables cloud-native deployment and event-driven processing
