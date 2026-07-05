package permits

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// buildingReportRe matches building report PDF href paths on the weekly reports page.
// Example: /__shared/assets/buildingreportmarch15_202678649.pdf
var buildingReportRe = regexp.MustCompile(`/__shared/assets/buildingreport[^"]+\.pdf`)

// fetchPDFURLs scrapes the weekly reports page and returns absolute URLs for
// building report PDFs. Demolition reports are excluded by the regex.
func (r *RichmondCollector) fetchPDFURLs(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, richmondReportsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("richmond: build request: %w", err)
	}
	req.Header.Set("User-Agent", "groupscout-leadgen/1.0 (hotel group sales intelligence)")

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("richmond: fetch reports page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("richmond: reports page returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("richmond: read response body: %w", err)
	}

	paths := buildingReportRe.FindAllString(string(body), -1)
	if len(paths) == 0 {
		return nil, fmt.Errorf("richmond: no building report PDFs found — page structure may have changed")
	}

	seen := make(map[string]bool)
	urls := make([]string, 0, len(paths))
	for _, p := range paths {
		if !seen[p] {
			seen[p] = true
			urls = append(urls, richmondBaseURL+p)
		}
	}
	return urls, nil
}

// downloadPDF fetches a PDF from url, writes it to a temp file, and returns
// the file path, the raw bytes, plus a cleanup function the caller must defer.
func (r *RichmondCollector) downloadPDF(ctx context.Context, url string) (path string, data []byte, cleanup func(), err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", nil, nil, fmt.Errorf("richmond: build pdf request: %w", err)
	}
	req.Header.Set("User-Agent", "groupscout-leadgen/1.0 (hotel group sales intelligence)")

	resp, err := r.client.Do(req)
	if err != nil {
		return "", nil, nil, fmt.Errorf("richmond: download pdf: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", nil, nil, fmt.Errorf("richmond: pdf download returned HTTP %d", resp.StatusCode)
	}

	data, err = io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, nil, fmt.Errorf("richmond: read pdf body: %w", err)
	}

	tmp, err := os.CreateTemp("", "richmond-*.pdf")
	if err != nil {
		return "", nil, nil, fmt.Errorf("richmond: create temp file: %w", err)
	}

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", nil, nil, fmt.Errorf("richmond: write pdf: %w", err)
	}
	tmp.Close()

	return tmp.Name(), data, func() { os.Remove(tmp.Name()) }, nil
}

// parsePDF extracts all permit records from a Richmond PDF report.
// It shells out to pdftotext (Poppler), which correctly handles Richmond's PDF font encoding.
// The plain-text output is split into lines and fed to parsePermitLines.
func parsePDF(path string) ([]permitRecord, error) {
	pdftotext, err := findPdftotext()
	if err != nil {
		return nil, err
	}

	out, err := exec.Command(pdftotext, path, "-").Output()
	if err != nil {
		return nil, fmt.Errorf("richmond: pdftotext: %w", err)
	}

	var lines []string
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		if line := strings.TrimSpace(scanner.Text()); line != "" {
			lines = append(lines, line)
		}
	}

	return parsePermitLines(lines), nil
}

// findPdftotext returns the path to the pdftotext binary.
// Checks the location bundled with Git for Windows first, then falls back to PATH.
func findPdftotext() (string, error) {
	const gitPath = `C:\Program Files\Git\mingw64\bin\pdftotext.exe`
	if _, err := os.Stat(gitPath); err == nil {
		return gitPath, nil
	}
	p, err := exec.LookPath("pdftotext")
	if err != nil {
		return "", fmt.Errorf("richmond: pdftotext not found — install Poppler or Git for Windows (expected at %s)", gitPath)
	}
	return p, nil
}
