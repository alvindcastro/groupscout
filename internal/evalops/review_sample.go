package evalops

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

var ErrUnsafeReviewSample = errors.New("unsafe review sample")

type ReviewSample struct {
	TraceID        string            `json:"trace_id"`
	CaseID         string            `json:"case_id,omitempty"`
	Stage          TraceStage        `json:"stage"`
	Severity       Severity          `json:"severity,omitempty"`
	Reason         string            `json:"reason"`
	SourceSystem   string            `json:"source_system,omitempty"`
	SourceType     string            `json:"source_type,omitempty"`
	CapturedAt     time.Time         `json:"captured_at,omitempty"`
	ReviewRequired bool              `json:"review_required"`
	Attributes     map[string]string `json:"attributes,omitempty"`
	Results        []Result          `json:"results,omitempty"`
}

type ReviewSampleWriterConfig struct {
	Enabled       bool
	Path          string
	MaxFieldBytes int
	Clock         func() time.Time
}

type ReviewSampleWriter struct {
	config ReviewSampleWriterConfig
}

func NewReviewSampleWriter(config ReviewSampleWriterConfig) *ReviewSampleWriter {
	if config.MaxFieldBytes <= 0 {
		config.MaxFieldBytes = 1024
	}
	if config.Clock == nil {
		config.Clock = func() time.Time { return time.Now().UTC() }
	}
	return &ReviewSampleWriter{config: config}
}

func (w *ReviewSampleWriter) Write(sample ReviewSample) error {
	if !w.config.Enabled {
		return nil
	}
	if w.config.Path == "" {
		return fmt.Errorf("review sample path is required")
	}
	if err := rejectUnsafeSample(sample); err != nil {
		return err
	}
	safe := sample.safe(w.config.MaxFieldBytes)
	if safe.CapturedAt.IsZero() {
		safe.CapturedAt = w.config.Clock().UTC()
	}
	safe.ReviewRequired = true
	data, err := json.Marshal(safe)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(w.config.Path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(w.config.Path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Write(append(data, '\n')); err != nil {
		return err
	}
	return nil
}

func (s ReviewSample) safe(maxFieldBytes int) ReviewSample {
	safe := s
	safe.Attributes = sanitizeStringMap(s.Attributes, maxFieldBytes)
	safe.Results = redactResults(s.Results)
	return safe
}

func rejectUnsafeSample(sample ReviewSample) error {
	for key, value := range sample.Attributes {
		if sensitiveAttributeKey(key) {
			return fmt.Errorf("%w: sensitive attribute key %q", ErrUnsafeReviewSample, key)
		}
		if redactSensitive(value) != value {
			return fmt.Errorf("%w: sensitive attribute value %q", ErrUnsafeReviewSample, key)
		}
	}
	return nil
}
