package permits

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/alvindcastro/groupscout/internal/collector"
	"github.com/alvindcastro/groupscout/internal/logger"
)

const (
	richmondBaseURL    = "https://www.richmond.ca"
	richmondReportsURL = "https://www.richmond.ca/business-development/building-approvals/reports/weeklyreports.htm"
)

// RichmondCollector scrapes building permit PDFs published weekly by the City of Richmond BC.
// Richmond has no open data API; data is only available as PDFs at:
// https://www.richmond.ca/business-development/building-approvals/reports/weeklyreports.htm
type RichmondCollector struct {
	client   *http.Client
	Verbose  bool  // when true, logs intermediate step counts to stderr
	MinValue int64 // minimum construction value to pass the filter (default: minPermitValueCAD)
}

// NewRichmondCollector returns a RichmondCollector with a 30-second HTTP timeout.
func NewRichmondCollector() *RichmondCollector {
	return &RichmondCollector{
		client:   &http.Client{Timeout: 30 * time.Second},
		MinValue: minPermitValueCAD,
	}
}

// Name satisfies the collector.Collector interface.
func (r *RichmondCollector) Name() string { return "richmond_permits" }

// Collect satisfies the collector.Collector interface. Downloads the most recent weekly
// building report PDF, parses all permit records, and returns them as RawProjects.
func (r *RichmondCollector) Collect(ctx context.Context) ([]collector.RawProject, error) {
	urls, err := r.fetchPDFURLs(ctx)
	if err != nil {
		return nil, err
	}
	if len(urls) == 0 {
		return nil, fmt.Errorf("richmond: no PDF URLs found")
	}

	// Always process only the most recent report (first link = latest week).
	if r.Verbose {
		logger.Log.Info("processing latest report", "source", "richmond", "count", len(urls), "url", urls[0])
	}

	path, rawData, cleanup, err := r.downloadPDF(ctx, urls[0])
	if err != nil {
		return nil, err
	}
	defer cleanup()

	records, err := parsePDF(path)
	if err != nil {
		return nil, err
	}

	if r.Verbose {
		logRichmondRecordCounts(records)
	}

	pdfURL := urls[0]
	var projects []collector.RawProject
	var skippedValue, skippedType int
	for _, rec := range records {
		if rec.ValueCAD <= r.MinValue {
			skippedValue++
			continue
		}
		if !isRelevantSubType(rec.SubType) {
			skippedType++
			continue
		}
		p := toRawProject(rec, rawData)
		p.SourceURL = pdfURL
		p.Hash = hashPermit(rec.FolderNumber, rec.Address, rec.IssueDate)
		projects = append(projects, p)
	}

	if r.Verbose {
		logger.Log.Info("filtering complete",
			"source", "richmond",
			"passed", len(projects),
			"skipped_low_value", skippedValue,
			"skipped_residential", skippedType,
			"min_value", r.MinValue,
		)
	}

	return projects, nil
}

func logRichmondRecordCounts(records []permitRecord) {
	logger.Log.Info("parsed records from PDF", "source", "richmond", "count", len(records))
	counts := make(map[string]int)
	for _, rec := range records {
		counts[rec.SubType]++
	}
	for subType, n := range counts {
		logger.Log.Debug("permits by sub-type", "source", "richmond", "sub_type", subType, "count", n)
	}
}
