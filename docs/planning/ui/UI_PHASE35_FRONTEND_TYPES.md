# Phase 35 Frontend Type Contract

> Temporary TypeScript contract for the future GroupScout frontend package.
> When a frontend package exists, these interfaces should move into generated or checked client types derived from `api/swagger.yaml`.

```ts
export interface LeadSummary {
  id: string;
  title: string;
  source: string;
  location: string;
  project_value: number;
  priority_score: number;
  priority_reason: string;
  status: string;
  created_at: string;
  updated_at: string;
  has_raw: boolean;
  audit_source_url?: string;
}

export interface LeadListFilters {
  status: string;
  source: string;
  min_score: number;
  q: string;
  limit: number;
  cursor: string;
}

export interface LeadListResponse {
  items: LeadSummary[];
  next_cursor: string;
  filters: LeadListFilters;
}

export interface LeadDetail {
  id: string;
  source: string;
  title: string;
  location: string;
  project_value: number;
  general_contractor: string;
  applicant: string;
  contractor: string;
  source_url: string;
  project_type: string;
  estimated_crew_size: number;
  estimated_duration_months: number;
  out_of_town_crew_likely: boolean;
  priority_score: number;
  priority_reason: string;
  rationale: string;
  suggested_outreach_timing: string;
  notes: string;
  status: string;
  created_at: string;
  updated_at: string;
}

export interface LeadAuditMetadata {
  has_raw: boolean;
  raw_link?: string;
  payload_type?: string;
  source_url?: string;
  collector_name?: string;
  collected_at?: string;
}

export interface OutreachSummary {
  count: number;
  latest_at: string | null;
}

export interface LeadDetailResponse {
  lead: LeadDetail;
  audit: LeadAuditMetadata;
  outreach_summary: OutreachSummary;
  activity: unknown[];
}

export interface LeadPatchRequest {
  status?: string;
  notes?: string;
}

export interface LeadPatchResponse {
  lead: LeadDetail;
  changed_fields: Array<"status" | "notes">;
  updated_at: string;
}

export interface APIError {
  error: string;
}
```

Unsafe or not-yet-schema-backed edit fields must not be added here until storage migrations and failing tests exist for them.

