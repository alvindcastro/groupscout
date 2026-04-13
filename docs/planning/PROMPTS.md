# Prompt Engineering & Management

**Goal:** Centralize, version, and rigorously test all LLM prompts used in GroupScout to ensure high-quality lead enrichment and scoring.

## Current Prompt Library

### System Prompt
**File:** `internal/enrichment/claude.go`
**Role:** Sets the persona and core constraints for the lead analyst.

```text
You are a lead analyst for the Sandman Hotel Vancouver Airport in Richmond, BC (near YVR).
You evaluate building permit records to identify projects that will generate demand for construction crew lodging.

Key factors to weigh:
- Project value and type: new builds and major industrial/commercial projects bring large out-of-town crews
- Location: projects within 30 km of Richmond BC are the target area
- Contractor identity: larger or out-of-province GCs typically bring travelling crews
- Duration: longer projects mean extended-stay demand (room blocks, direct billing, weekly rates)

Respond with ONLY a valid JSON object. No markdown, no explanation, no code fences.
```

### Building Permits (`permitPrompt`)
Used for Richmond, Delta, and general permit sources.

### Film/TV Productions (`creativeBCPrompt`)
Specific to Creative BC; focuses on crew size and US studio presence.

### Convention Centre Events (`vccEventsPrompt`)
Focuses on attendee counts and professional industries (medical, tech, etc.).

### News & Announcements (`newsPrompt`, `announcementsPrompt`)
Evaluates infrastructure signals from RSS/Google News.

### Outreach Drafting (`DraftOutreach`)
Generates cold emails for the sales team.

---

## Strict TDD for Prompts

To ensure prompts remain effective as the system evolves, we follow a strict TDD approach for any prompt changes.

### 1. Test Case Definition
Every prompt must have a corresponding test suite in `internal/enrichment/prompts_test.go` (to be created) that uses **Gold Standard Fixtures**.
- **Input:** A known `RawProject` (e.g., a $10M Richmond warehouse permit).
- **Expected Output:** Specific fields or scores (e.g., `PriorityScore` >= 8, `OutOfTownCrewLikely` == true).

### 2. Regression Testing
Any change to a prompt string requires running the full evaluation suite to ensure:
- JSON structure remains valid.
- Priority scoring hasn't drifted for known "perfect" leads.
- No "hallucinations" in the `GeneralContractor` field for known records.

### 3. Edge Case Coverage
Test prompts against:
- Extremely low-value permits (should score 1-2).
- Permits with missing descriptions.
- Projects far outside the 30km radius (should score 1-3).
- Duplicated inputs (should remain consistent).

## Planned Improvements (Phase 29)

- [ ] **Prompt Template Migration:** Move prompts out of `.go` files and into `assets/prompts/*.tmpl` for better visibility and hot-reloading.
- [ ] **Few-Shot Examples:** Inject 2-3 "Gold Standard" examples into the user turn to improve scoring accuracy.
- [ ] **Evaluation Harness:** Automate the comparison between "Human Score" and "AI Score" in the test suite.
