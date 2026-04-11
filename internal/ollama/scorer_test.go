package ollama

import (
	"context"
	"strings"
	"testing"

	"github.com/alvindcastro/groupscout/internal/storage"
)

func TestScorer_Rationale(t *testing.T) {
	tests := []struct {
		name         string
		lead         storage.Lead
		mockResponse string
		mockErr      error
		wantContains string
		wantLenMax   int
		wantErr      error
	}{
		{
			name: "happy path",
			lead: storage.Lead{
				Title:         "ABC Construction",
				ProjectType:   "construction",
				Location:      "Richmond, BC",
				PriorityScore: 8,
			},
			mockResponse: "This is a strong construction lead in Richmond with high priority. The sales team should reach out immediately to discuss room blocks for the crew.",
			wantContains: "Richmond",
			wantLenMax:   280,
		},
		{
			name: "truncation",
			lead: storage.Lead{
				Title: "Long Rationale Project",
			},
			mockResponse: strings.Repeat("This is a very long rationale that should definitely be truncated because it exceeds the maximum allowed length of two hundred and eighty characters by quite a substantial margin. We need to make sure the truncation logic works correctly and cuts at the last complete word. ", 2),
			wantLenMax:   280,
		},
		{
			name:         "empty lead",
			lead:         storage.Lead{},
			mockResponse: "No information available for this lead.",
			wantLenMax:   280,
		},
		{
			name:    "context cancellation",
			lead:    storage.Lead{Title: "Cancelled"},
			mockErr: context.Canceled,
			wantErr: context.Canceled,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockLLMClient{
				chatCompleteFunc: func(ctx context.Context, system, user string) (string, error) {
					if tt.mockErr != nil {
						return "", tt.mockErr
					}
					return tt.mockResponse, nil
				},
			}
			s := NewScorer(mock)
			got, err := s.Rationale(context.Background(), tt.lead)

			if tt.wantErr != nil {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr.Error()) {
					t.Errorf("Rationale() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Errorf("Rationale() unexpected error = %v", err)
				return
			}

			if tt.wantContains != "" && !strings.Contains(got, tt.wantContains) {
				t.Errorf("Rationale() = %v, want it to contain %v", got, tt.wantContains)
			}

			if tt.wantLenMax > 0 && len(got) > tt.wantLenMax {
				t.Errorf("Rationale() length = %d, want <= %d", len(got), tt.wantLenMax)
			}

			if tt.name == "truncation" {
				if len(got) > 280 {
					t.Errorf("Truncation failed: length %d > 280", len(got))
				}
				if strings.HasSuffix(got, " ") {
					t.Errorf("Truncation should not end with a space: %q", got)
				}
				// Verify it ends at a word boundary (simplified check)
				lastChar := got[len(got)-1:]
				if lastChar == " " {
					t.Errorf("Truncation ended on a space")
				}
			}
		})
	}
}
