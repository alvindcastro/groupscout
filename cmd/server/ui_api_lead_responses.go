package main

import (
	"time"

	"github.com/alvindcastro/groupscout/internal/storage"
)

type leadSummaryResponse struct {
	ID             string    `json:"id"`
	Title          string    `json:"title"`
	Source         string    `json:"source"`
	Location       string    `json:"location"`
	ProjectValue   int64     `json:"project_value"`
	PriorityScore  int       `json:"priority_score"`
	PriorityReason string    `json:"priority_reason"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	HasRaw         bool      `json:"has_raw"`
	AuditSourceURL string    `json:"audit_source_url,omitempty"`
}

type leadDetailResponse struct {
	ID                      string     `json:"id"`
	Source                  string     `json:"source"`
	Title                   string     `json:"title"`
	Location                string     `json:"location"`
	ProjectValue            int64      `json:"project_value"`
	GeneralContractor       string     `json:"general_contractor"`
	Applicant               string     `json:"applicant"`
	Contractor              string     `json:"contractor"`
	SourceURL               string     `json:"source_url"`
	ProjectType             string     `json:"project_type"`
	EstimatedCrewSize       int        `json:"estimated_crew_size"`
	EstimatedDurationMonths int        `json:"estimated_duration_months"`
	OutOfTownCrewLikely     bool       `json:"out_of_town_crew_likely"`
	PriorityScore           int        `json:"priority_score"`
	PriorityReason          string     `json:"priority_reason"`
	Rationale               string     `json:"rationale"`
	SuggestedOutreachTiming string     `json:"suggested_outreach_timing"`
	Notes                   string     `json:"notes"`
	Owner                   string     `json:"owner"`
	SnoozedUntil            *time.Time `json:"snoozed_until"`
	Flagged                 bool       `json:"flagged"`
	VerificationState       string     `json:"verification_state"`
	Status                  string     `json:"status"`
	CreatedAt               time.Time  `json:"created_at"`
	UpdatedAt               time.Time  `json:"updated_at"`
}

func leadSummary(lead storage.Lead) leadSummaryResponse {
	return leadSummaryResponse{
		ID:             lead.ID,
		Title:          lead.Title,
		Source:         lead.Source,
		Location:       lead.Location,
		ProjectValue:   lead.ProjectValue,
		PriorityScore:  lead.PriorityScore,
		PriorityReason: lead.PriorityReason,
		Status:         lead.Status,
		CreatedAt:      lead.CreatedAt,
		UpdatedAt:      lead.UpdatedAt,
		HasRaw:         lead.RawInputID != "",
		AuditSourceURL: lead.SourceURL,
	}
}

func leadDetail(lead storage.Lead) leadDetailResponse {
	return leadDetailResponse{
		ID:                      lead.ID,
		Source:                  lead.Source,
		Title:                   lead.Title,
		Location:                lead.Location,
		ProjectValue:            lead.ProjectValue,
		GeneralContractor:       lead.GeneralContractor,
		Applicant:               lead.Applicant,
		Contractor:              lead.Contractor,
		SourceURL:               lead.SourceURL,
		ProjectType:             lead.ProjectType,
		EstimatedCrewSize:       lead.EstimatedCrewSize,
		EstimatedDurationMonths: lead.EstimatedDurationMonths,
		OutOfTownCrewLikely:     lead.OutOfTownCrewLikely,
		PriorityScore:           lead.PriorityScore,
		PriorityReason:          lead.PriorityReason,
		Rationale:               lead.Rationale,
		SuggestedOutreachTiming: lead.SuggestedOutreachTiming,
		Notes:                   lead.Notes,
		Owner:                   lead.Owner,
		SnoozedUntil:            lead.SnoozedUntil,
		Flagged:                 lead.Flagged,
		VerificationState:       lead.VerificationState,
		Status:                  lead.Status,
		CreatedAt:               lead.CreatedAt,
		UpdatedAt:               lead.UpdatedAt,
	}
}
