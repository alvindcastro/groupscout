### Future Integration Roadmap

This document maps future integration ideas to the existing [ARCHITECTURE.md](./ARCHITECTURE.md) to define a development roadmap for GroupScout.

---

### 1. Agentic Engineering & GenAI Workflows
*Target: `internal/enrichment`*

- [ ] **Reasoning Loops** — Evolve from simple single-call Claude API enrichment to multi-step reasoning (ReAct/Plan-and-Solve) for complex project analysis.
- [ ] **RAG Implementation** — Add a vector database (e.g., ChromaDB, Pinecone, or Vertex AI Vector Search) to provide context-aware project matching against historical successful leads.
- [ ] **Tool-Calling** — Enable agents to use tools (e.g., searching LinkedIn, verifying business registrations) during the enrichment phase.

### 2. Data Foundation & AI Pipelines
*Target: `internal/collector`, `internal/storage`*

- [ ] **Unstructured Ingestion** — Expand collectors to handle PDF tender documents and complex logs, using AI-driven parsing to extract structured `RawProject` data.
- [ ] **AI-Ready SQL** — Implement advanced SQL/dbt logic in the storage layer to pre-aggregate and clean data, ensuring it is optimized for LLM consumption.
- [ ] **AI Observability** — Integrate frameworks like RAGAS or Vertex Eval to monitor for hallucinations and track the quality of AI-generated outreach drafts.

### 3. Integration & Cloud-Native Development
*Target: `config`, `infrastructure`*

- [ ] **AIaaS API Layer** — Build out the existing Go REST API into a robust Inference Layer, exposing AI enrichment capabilities as a standalone service.
- [ ] **Infrastructure as Code (IaC)** — Provide Terraform templates to deploy the entire GroupScout stack on Google Cloud (Vertex AI, Cloud Run, Cloud SQL).
- [ ] **Event-Driven Architecture** — Transition from scheduled cron jobs to a Pub/Sub or Webhook-triggered model (e.g., using Google Cloud Workflows) for real-time lead processing.
- [ ] **CRM/ERP Integration** — Connect `internal/notify` to common business tools (HubSpot, Salesforce, SAP) via secure API orchestration to allow agents to directly create records or trigger actions.

### 4. Agile Execution & Quality
*Target: `docs/TESTING.md`, `internal/notify`*

- [ ] **Automated UAT** — Develop automated User Acceptance Testing (UAT) suites to validate the business value of AI-generated endpoints and drafts.
- [ ] **Technical Validation** — Implement stricter validation rules (DoD) for lead scoring, ensuring technical trade-offs (speed vs. scalability) are documented for each enrichment model.

---

### Summary of Next Steps

- [ ] **Pilot Agentic Reasoning** — Prototype a `ReAct` loop in `internal/enrichment` using Claude Sonnet.
- [ ] **Vector Search POC** — Set up a local vector store for RAG-based context enrichment.
- [ ] **Terraform Scaffolding** — Create basic IaC configurations for cloud deployment.
