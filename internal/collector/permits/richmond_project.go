package permits

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	"github.com/alvindcastro/groupscout/internal/collector"
)

// minPermitValueCAD is the minimum construction value for a permit to be considered.
// Residential and low-value permits rarely involve out-of-town crews.
const minPermitValueCAD = 500_000

// commercialSubTypes is the whitelist of permit sub-types relevant to hotel group sales.
// Residential sub-types (One Family Dwelling, Townhouse, etc.) are excluded because they
// do not generate construction crew lodging demand at scale.
var commercialSubTypes = map[string]bool{
	"hotel":                   true,
	"warehouse":               true,
	"manufacturing/warehouse": true, // Richmond uses this combined sub-type
	"office":                  true,
	"medical office":          true,
	"dental office":           true,
	"financial institute":     true, // banks, credit unions; large TI projects
	"restaurant":              true,
	"retail":                  true,
	"apartment":               true,
	"educational facility":    true,
	"community hall":          true,
	"recreational":            true,
	"industrial":              true,
	"canopy":                  true,
	"nursing home":            true, // extended renovation projects, multi-trade crews
}

func isRelevantSubType(subType string) bool {
	return commercialSubTypes[strings.ToLower(strings.TrimSpace(subType))]
}

// isRelevant returns true if a permit record is worth enriching.
// Filters out residential sub-types and permits strictly below minValue.
func isRelevant(rec permitRecord, minValue int64) bool {
	if rec.ValueCAD <= minValue {
		return false
	}
	return isRelevantSubType(rec.SubType)
}

// toRawProject maps a permitRecord to the normalized collector.RawProject used by the pipeline.
func toRawProject(rec permitRecord, rawData []byte) collector.RawProject {
	return collector.RawProject{
		Source:     "richmond_permits",
		ExternalID: rec.FolderNumber,
		Title:      fmt.Sprintf("%s — %s", rec.SubType, rec.Address),
		Location:   rec.Address,
		Value:      rec.ValueCAD,
		Description: fmt.Sprintf(
			"Work: %s | Status: %s | Applicant: %s | Contractor: %s",
			rec.WorkProposed, rec.Status, rec.Applicant, rec.Contractor,
		),
		IssuedAt: rec.IssueDate,
		RawData:  rawData,
		RawType:  "application/pdf",
		Metadata: map[string]any{
			"applicant":  rec.Applicant,
			"contractor": rec.Contractor,
		},
	}
}

// formatCAD formats an int64 dollar amount with comma separators for log output.
func formatCAD(n int64) string {
	s := fmt.Sprintf("%d", n)
	out := make([]byte, 0, len(s)+len(s)/3)
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, byte(c))
	}
	return string(out)
}

// hashPermit produces a deterministic dedup key for a Richmond permit.
// Uses folder number (unique per permit) + address + date to guard against
// re-processing the same permit if it appears in multiple weekly reports.
func hashPermit(folderNumber, address string, issuedAt time.Time) string {
	h := sha256.Sum256([]byte(
		"richmond_permits|" + folderNumber + "|" + address + "|" + issuedAt.Format("2006-01-02"),
	))
	return fmt.Sprintf("%x", h)
}
