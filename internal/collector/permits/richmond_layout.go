package permits

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"math"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Richmond weekly building reports are one table per SUB_TYPE section:
//
//	SUB_TYPE: Warehouse
//	FOLDER_NUMBER WORK_PROPOSED STATUS ISSUE_DATE CONSTR_VALUE FOLDER_NAME APPLICANT CONTRACTOR
//	26 024034 000 00 B7   Alteration   Issued   2026/10/09   $2,000,000.00   2471 Viking Way   Jack Zhang ...   Chandos ...
//
// Every cell wraps onto several lines (the folder number, the date and the
// addresses all split), so plain pdftotext output interleaves cells unpredictably.
// pdftotext -bbox-layout instead emits one <block> per table cell with its
// coordinates, which is what this parser reads.

var (
	// layoutSubTypeRe matches section headers, "SUB_TYPE: Hotel" (and the older "SUB TYPE: Hotel").
	layoutSubTypeRe = regexp.MustCompile(`(?i)^SUB[_\s]TYPE:\s*(.+)$`)
	// layoutFolderRe matches a folder number cell, "26 021487 000 00 B8" or "26 014001 000 00 BB".
	layoutFolderRe = regexp.MustCompile(`^\d{2} \d{6}( \d{3})?( \d{2})?( [A-Z][A-Z0-9]*)?$`)
	// layoutDateRe matches an issue date cell once its wrapped lines are joined, "2026/10/08".
	layoutDateRe = regexp.MustCompile(`^\d{4}/\d{2}/\d{2}$`)
	// layoutValueRe matches a construction value cell, "$150,000.00".
	layoutValueRe = regexp.MustCompile(`^\$[\d,]+(\.\d{2})?$`)
	// layoutCountRe matches the per-section permit count printed on the subtotal row.
	layoutCountRe = regexp.MustCompile(`^\d+$`)
)

// layoutHeaders are the column header labels; underscores and spaces are both accepted.
var layoutHeaders = []string{"FOLDER_NUMBER", "WORK_PROPOSED", "STATUS", "ISSUE_DATE", "CONSTR_VALUE", "FOLDER_NAME", "APPLICANT", "CONTRACTOR"}

// bboxDoc mirrors the XHTML written by pdftotext -bbox-layout.
type bboxDoc struct {
	Pages []struct {
		Flows []struct {
			Blocks []bboxBlock `xml:"block"`
		} `xml:"flow"`
	} `xml:"body>doc>page"`
}

type bboxBlock struct {
	XMin  float64 `xml:"xMin,attr"`
	YMin  float64 `xml:"yMin,attr"`
	XMax  float64 `xml:"xMax,attr"`
	YMax  float64 `xml:"yMax,attr"`
	Lines []struct {
		Words []bboxWord `xml:"word"`
	} `xml:"line"`
}

type bboxWord struct {
	XMin float64 `xml:"xMin,attr"`
	Text string  `xml:",chardata"`
}

// layoutCell is one table cell: its box and its text with wrapped lines joined.
type layoutCell struct {
	xMin, yMin, xMax, yMax float64
	text                   string
	words                  []bboxWord
}

func (c layoutCell) yMid() float64 { return (c.yMin + c.yMax) / 2 }

// columnAnchors holds the x position of each column header in a section.
type columnAnchors map[string]float64

// layoutRow collects the cells that belong to one permit.
type layoutRow struct {
	rec     permitRecord
	folder  layoutCell
	anchors columnAnchors
	left    []layoutCell // WORK_PROPOSED / STATUS cells
	value   *layoutCell
	right   map[string][]layoutCell // FOLDER_NAME / APPLICANT / CONTRACTOR cells
}

// parsePDF extracts all permit records from a Richmond PDF report.
// It shells out to pdftotext (Poppler) for word coordinates.
func parsePDF(path string) ([]permitRecord, error) {
	pdftotext, err := findPdftotext()
	if err != nil {
		return nil, err
	}

	out, err := exec.Command(pdftotext, "-bbox-layout", path, "-").Output()
	if err != nil {
		return nil, fmt.Errorf("richmond: pdftotext: %w", err)
	}

	return parseBBoxLayout(out)
}

// parseBBoxLayout converts pdftotext -bbox-layout output into permit records.
//
// Cells are classified by content where that is unambiguous (folder number,
// issue date, dollar value) and by position otherwise. Each permit's cells are
// vertically centred on its row, so every cell is attached to the folder-number
// cell whose centre is nearest. The work/status pair is told apart by order,
// because those two headers do not always sit over their data; the right-hand
// text columns are split at the midpoints between their header labels, which do.
func parseBBoxLayout(data []byte) ([]permitRecord, error) {
	var doc bboxDoc
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.Strict = false
	dec.AutoClose = xml.HTMLAutoClose
	dec.Entity = xml.HTMLEntity
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("richmond: parse bbox layout: %w", err)
	}

	var records []permitRecord
	var subType string
	var anchors columnAnchors

	for _, page := range doc.Pages {
		var cells []layoutCell
		for _, flow := range page.Flows {
			for _, b := range flow.Blocks {
				cells = append(cells, toLayoutCell(b))
			}
		}
		sort.SliceStable(cells, func(i, j int) bool { return cells[i].yMin < cells[j].yMin })

		// Pass 1: walk top to bottom, tracking the section and its header, and
		// open a row for every folder number cell.
		var rows []*layoutRow
		var others []layoutCell
		for _, c := range cells {
			if m := layoutSubTypeRe.FindStringSubmatch(c.text); m != nil {
				subType = strings.TrimSpace(m[1])
				anchors = nil
				continue
			}
			if isHeaderCell(c) {
				if anchors == nil {
					anchors = columnAnchors{}
				}
				for _, w := range c.words {
					label := strings.ToUpper(strings.TrimSpace(w.Text))
					for _, h := range layoutHeaders {
						if label == h {
							anchors[h] = w.XMin
						}
					}
				}
				continue
			}
			if layoutFolderRe.MatchString(c.text) {
				rows = append(rows, &layoutRow{
					rec:     permitRecord{SubType: subType, FolderNumber: c.text},
					folder:  c,
					anchors: anchors,
					right:   map[string][]layoutCell{},
				})
				continue
			}
			others = append(others, c)
		}

		// Pass 2: attach every other cell to the nearest row.
		for _, c := range others {
			row := nearestRow(rows, c)
			if row == nil {
				continue
			}
			compact := strings.ReplaceAll(c.text, " ", "")
			switch {
			case layoutDateRe.MatchString(compact):
				if t, err := time.Parse("2006/01/02", compact); err == nil {
					row.rec.IssueDate = t
				}
			case layoutValueRe.MatchString(c.text):
				// The section subtotal row also carries a value; keep the cell
				// closest to the row centre.
				if row.value == nil || math.Abs(c.yMid()-row.folder.yMid()) < math.Abs(row.value.yMid()-row.folder.yMid()) {
					cc := c
					row.value = &cc
				}
			case layoutCountRe.MatchString(c.text):
				// subtotal count or page number
			default:
				switch col := textColumn(row, c); col {
				case "":
				case "LEFT":
					row.left = append(row.left, c)
				default:
					row.right[col] = append(row.right[col], c)
				}
			}
		}

		for _, row := range rows {
			records = append(records, row.finish())
		}
	}

	return records, nil
}

func toLayoutCell(b bboxBlock) layoutCell {
	c := layoutCell{xMin: b.XMin, yMin: b.YMin, xMax: b.XMax, yMax: b.YMax}
	var lines []string
	for _, l := range b.Lines {
		var words []string
		for _, w := range l.Words {
			words = append(words, strings.TrimSpace(w.Text))
			c.words = append(c.words, w)
		}
		lines = append(lines, strings.Join(words, " "))
	}
	c.text = strings.TrimSpace(strings.Join(lines, " "))
	return c
}

// isHeaderCell reports whether a cell holds column header labels.
func isHeaderCell(c layoutCell) bool {
	upper := strings.ToUpper(strings.ReplaceAll(c.text, " ", "_"))
	for _, h := range layoutHeaders {
		if strings.Contains(upper, h) {
			return true
		}
	}
	return false
}

// nearestRow returns the row whose folder-number cell is vertically closest to
// c, provided c overlaps that row's band; cells from subtotal rows, page
// headers and footers fall outside every band and return nil.
func nearestRow(rows []*layoutRow, c layoutCell) *layoutRow {
	var best *layoutRow
	bestDist := math.MaxFloat64
	for _, r := range rows {
		d := math.Abs(c.yMid() - r.folder.yMid())
		if d < bestDist {
			best, bestDist = r, d
		}
	}
	if best == nil {
		return nil
	}
	// A row's cells share its centre line, give or take one wrapped line.
	tolerance := (best.folder.yMax-best.folder.yMin)/2 + 6
	if bestDist > tolerance {
		return nil
	}
	return best
}

// textColumn decides which column a free-text cell belongs to.
func textColumn(row *layoutRow, c layoutCell) string {
	a := row.anchors
	date, ok := a["ISSUE_DATE"]
	if !ok {
		return ""
	}
	if c.xMin < date-5 {
		return "LEFT"
	}
	name, okName := a["FOLDER_NAME"]
	applicant, okApplicant := a["APPLICANT"]
	contractor, okContractor := a["CONTRACTOR"]
	if !okName || !okApplicant || !okContractor || c.xMin < name-20 {
		return ""
	}
	switch {
	case c.xMin < (name+applicant)/2:
		return "FOLDER_NAME"
	case c.xMin < (applicant+contractor)/2:
		return "APPLICANT"
	default:
		return "CONTRACTOR"
	}
}

// knownStatuses are the permit statuses Richmond prints; used when a row has
// only one of the work/status cells.
var knownStatuses = map[string]bool{"issued": true, "completed": true, "cancelled": true, "expired": true, "closed": true}

func (r *layoutRow) finish() permitRecord {
	rec := r.rec
	if r.value != nil {
		rec.ValueCAD = parseDollarAmount(r.value.text)
	}

	sort.SliceStable(r.left, func(i, j int) bool { return r.left[i].xMin < r.left[j].xMin })
	switch len(r.left) {
	case 0:
	case 1:
		if knownStatuses[strings.ToLower(r.left[0].text)] {
			rec.Status = r.left[0].text
		} else {
			rec.WorkProposed = r.left[0].text
		}
	default:
		rec.WorkProposed = joinCells(r.left[:len(r.left)-1])
		rec.Status = r.left[len(r.left)-1].text
	}

	rec.Address = joinCells(r.right["FOLDER_NAME"])
	rec.Applicant = joinCells(r.right["APPLICANT"])
	rec.Contractor = joinCells(r.right["CONTRACTOR"])
	return rec
}

func joinCells(cells []layoutCell) string {
	sort.SliceStable(cells, func(i, j int) bool { return cells[i].yMin < cells[j].yMin })
	parts := make([]string, 0, len(cells))
	for _, c := range cells {
		parts = append(parts, c.text)
	}
	return strings.Join(parts, " ")
}
