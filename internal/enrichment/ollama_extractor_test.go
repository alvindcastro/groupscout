package enrichment

import (
	"context"
	"os"
	"testing"
	"time"
)

type mockLLMClient struct {
	chatCompleteFunc func(ctx context.Context, system, user string) (string, error)
}

func (m *mockLLMClient) Generate(ctx context.Context, prompt string) (string, error) {
	return "", nil
}

func (m *mockLLMClient) ChatComplete(ctx context.Context, system, user string) (string, error) {
	return m.chatCompleteFunc(ctx, system, user)
}

func (m *mockLLMClient) ListModels(ctx context.Context) ([]string, error) {
	return nil, nil
}

func (m *mockLLMClient) HealthCheck(ctx context.Context) error {
	return nil
}

func TestExtractor_Extract(t *testing.T) {
	permitRaw, _ := os.ReadFile("testdata/permit_raw.txt")
	newsRaw, _ := os.ReadFile("testdata/news_raw.txt")

	tests := []struct {
		name         string
		rawText      string
		mockResponse string
		mockErr      error
		wantOrg      string
		wantType     string
		wantConf     float64
		wantErr      bool
	}{
		{
			name:    "permit extraction",
			rawText: string(permitRaw),
			mockResponse: `{
				"org_name": "ABC Construction Ltd.",
				"location": "1234 W Georgia St, Vancouver, BC",
				"start_date": "2026-05-01",
				"duration_days": 120,
				"crew_size": 45,
				"project_type": "construction",
				"confidence": 0.95
			}`,
			wantOrg:  "ABC Construction Ltd.",
			wantType: "construction",
			wantConf: 0.95,
		},
		{
			name:    "news extraction with markdown fences",
			rawText: string(newsRaw),
			mockResponse: "```json\n" + `{
				"org_name": "SkyLine Systems",
				"location": "Yaletown, Vancouver, BC",
				"start_date": "2026-06-15",
				"duration_days": 540,
				"crew_size": 12,
				"project_type": "government_contract",
				"confidence": 0.88
			}` + "\n```",
			wantOrg:  "SkyLine Systems",
			wantType: "government_contract",
			wantConf: 0.88,
		},
		{
			name:         "malformed JSON",
			rawText:      "some text",
			mockResponse: `{"org_name": "Incomplete...`,
			wantErr:      false,
			wantOrg:      "",
		},
		{
			name:    "client error",
			rawText: "some text",
			mockErr: context.DeadlineExceeded,
			wantErr: false,
			wantOrg: "",
		},
		{
			name:    "null fields handling",
			rawText: "some text",
			mockResponse: `{
				"org_name": null,
				"location": null,
				"start_date": null,
				"duration_days": null,
				"crew_size": null,
				"project_type": "other",
				"confidence": 0.5
			}`,
			wantOrg:  "",
			wantType: "other",
			wantConf: 0.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockLLMClient{
				chatCompleteFunc: func(ctx context.Context, system, user string) (string, error) {
					return tt.mockResponse, tt.mockErr
				},
			}
			e := NewExtractor(mock)
			got, err := e.Extract(context.Background(), tt.rawText)
			if (err != nil) != tt.wantErr {
				t.Errorf("Extract() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got == nil {
					if tt.wantOrg != "" {
						t.Errorf("Extract() returned nil, want signal with org %s", tt.wantOrg)
					}
					return
				}
				if got.OrgName != tt.wantOrg {
					t.Errorf("OrgName = %v, want %v", got.OrgName, tt.wantOrg)
				}
				if got.ProjectType != tt.wantType {
					t.Errorf("ProjectType = %v, want %v", got.ProjectType, tt.wantType)
				}
				if got.Confidence != tt.wantConf {
					t.Errorf("Confidence = %v, want %v", got.Confidence, tt.wantConf)
				}
				// Verify dates if expected
				if tt.name == "permit extraction" {
					wantDate := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
					if !got.StartDate.Equal(wantDate) {
						t.Errorf("StartDate = %v, want %v", got.StartDate, wantDate)
					}
				}
			}
		})
	}
}
