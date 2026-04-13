# Phase 30 — Advanced Prompts (Verification & Source Analysis)

This file contains the core prompt templates used for AI-driven verification and source analysis in Phase 30.

## 1. AI Verification Prompt

Used to cross-reference an extracted `Lead` JSON against its original `RawInput` payload to find hallucinations or misinterpretations.

**Model Goal:** Find discrepancies between the structured lead data and the source text.

```text
### Verification Prompt Template

System: You are an expert data auditor for a hotel sales intelligence system. Your goal is to compare a structured JSON lead (the extraction) against the original raw source text (the source material) to verify its accuracy.

Input:
1. Structured Lead (JSON): {{.LeadJSON}}
2. Raw Source Text (Text/Markdown): {{.RawSourceText}}

Instructions:
1. For each field in the Structured Lead, find the corresponding evidence in the Raw Source Text.
2. Flag any "hallucinations" (information in the JSON not found in the source).
3. Flag any "misinterpretations" (information in the JSON that contradicts the source).
4. Identify any "missed high-value data" (important details in the source not captured in the JSON).
5. Assign a "Confidence Score" (0.0 - 1.0) for each field.

Output Format (JSON):
{
  "verification_score": 0.0 - 1.0 (overall accuracy),
  "discrepancies": [
    {
      "field": "string",
      "type": "hallucination | contradiction | omission",
      "found_in_json": "string",
      "found_in_source": "string",
      "severity": "low | medium | high"
    }
  ],
  "field_confidence": {
    "title": 0.0 - 1.0,
    "location": 0.0 - 1.0,
    "project_value": 0.0 - 1.0,
    "contractor": 0.0 - 1.0
  },
  "overall_summary": "string"
}
```

## 2. Source Change Analysis Prompt

Used when the same URL/source is scraped multiple times to identify project updates or permit revisions.

```text
### Source Change Analysis Prompt Template

System: You are a construction project analyst. Your goal is to identify what has changed between two versions of the same raw data snapshot (e.g., a permit update).

Input:
1. Previous Raw Text: {{.PreviousRaw}}
2. Current Raw Text: {{.CurrentRaw}}

Instructions:
1. Ignore minor formatting or timestamp changes.
2. Focus on substantive project changes: value increases/decreases, new contractors added, status changes (e.g., "pending" to "issued"), or address revisions.
3. Summarize the changes in a way that helps a hotel sales team understand if the lead is now MORE or LESS valuable.

Output Format (JSON):
{
  "has_substantive_changes": boolean,
  "change_type": "value_update | status_change | contractor_added | address_revision | none",
  "summary": "Short, sales-focused summary of changes",
  "old_value": "string",
  "new_value": "string"
}
```

## 3. PDF/HTML Drift Detection Prompt

Used to detect if the source's structural layout has changed, which might break existing collectors.

```text
### Drift Detection Prompt Template

System: You are a system administrator monitoring web scrapers. Your goal is to detect if the structural layout of a source (HTML or PDF) has changed significantly from its known format.

Input:
1. Gold Standard Layout (Reference): {{.GoldStandardReference}}
2. New Scraped Content: {{.NewScrapedContent}}

Instructions:
1. Compare the headers, column order, and data patterns.
2. If the data is no longer appearing in expected locations, flag it as a "structural drift".

Output Format (JSON):
{
  "drift_detected": boolean,
  "confidence": 0.0 - 1.0,
  "details": "Description of what changed (e.g., 'Column Applicant moved from 3rd to 5th position')"
}
```
