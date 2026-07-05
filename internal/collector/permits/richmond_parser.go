package permits

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// folderNumRe identifies the start of a new permit record.
// Folder numbers follow the pattern: 25 036523 000 00 B7
var folderNumRe = regexp.MustCompile(`^\d{2}\s+\d{6}`)

// subTypeRe matches SUB TYPE section headers, e.g. "SUB TYPE: Hotel"
var subTypeRe = regexp.MustCompile(`(?i)SUB\s+TYPE:\s*(.+)`)

// skipLineRe matches lines that are column headers, totals, or page noise to ignore during parsing.
// Note: "applicant" and "contractor" are intentionally excluded; they are handled separately
// as right-column contact block markers, not skipped.
var skipLineRe = regexp.MustCompile(`(?i)^(folder\s*number|work\s*proposed|status|issue\s*date|constr|sub\s*total|grand\s*total|building\s*permit|city\s*of\s*richmond|filters|issued\s*from)`)

// issueDateRe matches YYYY/MM/DD date strings.
var issueDateRe = regexp.MustCompile(`^\d{4}/\d{2}/\d{2}$`)

// issueDateHeaderRe matches "ISSUE DATE 2026/03/18", a date embedded on the ISSUE DATE header
// line. Richmond PDFs for multi-permit sections output the first permit's date on the same line
// as the ISSUE DATE column header. Must be checked before skipLineRe, which discards the whole line.
var issueDateHeaderRe = regexp.MustCompile(`(?i)^issue\s+date\s+(\d{4}/\d{2}/\d{2})$`)

// issueDateWithCountRe matches "2026/03/18 2", a date followed by a whitespace-separated permit
// count. Richmond PDFs for multi-permit sections print subsequent dates merged with the section
// count on the same line (e.g. "2026/03/18 2" means date=2026/03/18, count=2 permits in section).
var issueDateWithCountRe = regexp.MustCompile(`^(\d{4}/\d{2}/\d{2})\s+\d+$`)

// valueRe matches dollar amounts like $300,000.00
var valueRe = regexp.MustCompile(`^\$[\d,]+\.?\d*$`)

// folderSuffixRe matches the trailing type code of a Richmond folder number (e.g. "B7").
// pdftotext sometimes wraps the folder number across two lines; this code appears alone on the second line.
var folderSuffixRe = regexp.MustCompile(`^[A-Z]\d+$`)

// folderNumExtractRe matches a complete folder number within a line.
// Used to split "19 878924 000 02 B7 Special Inspection Issued" into number + rest.
var folderNumExtractRe = regexp.MustCompile(`\d{2}\s+\d{6}\s+\d{3}\s+\d{2}\s+[A-Z]\d+`)

// folderNameHeaderRe matches "FOLDER NAME" as a standalone header line.
var folderNameHeaderRe = regexp.MustCompile(`(?i)^folder\s+name\s*$`)

// folderNameDataRe matches "FOLDER NAME 8640 Alexandra Road", label + address on one line.
var folderNameDataRe = regexp.MustCompile(`(?i)^folder\s+name\s+(.+)`)

// permitCountRe matches a lone integer printed as a row count in the PDF (e.g. "1").
var permitCountRe = regexp.MustCompile(`^\d+$`)

// applicantLineRe matches the APPLICANT right-column label (with optional inline name).
// Richmond PDFs render contact info in a separate right column; pdftotext outputs all
// permit records (left column) first, then all APPLICANT/CONTRACTOR blocks (right column).
var applicantLineRe = regexp.MustCompile(`(?i)^APPLICANT\s*(.*)`)

// contractorLineRe matches the CONTRACTOR right-column label (with optional inline name).
var contractorLineRe = regexp.MustCompile(`(?i)^CONTRACTOR\s*(.*)`)

// permitRecord holds the raw fields extracted from one permit entry in a Richmond PDF report.
// Fields are detected by content (date format, dollar sign, FOLDER NAME prefix, etc.)
// rather than position; the actual pdftotext output includes extra lines (permit count
// integers, column header repeats) that make positional parsing unreliable.
// SectionIndex is used internally to associate the right-column APPLICANT/CONTRACTOR
// contact blocks (which appear after all permit records on each page) with the correct permit.
type permitRecord struct {
	SubType      string // e.g. "Hotel", "Warehouse", "Office"
	FolderNumber string // e.g. "25 036523 000 00 B7"
	WorkProposed string // e.g. "New", "Alteration", "Revision"
	Status       string // e.g. "Issued"
	IssueDate    time.Time
	ValueCAD     int64  // construction value in CAD dollars
	Address      string // civic address + project description
	Applicant    string
	Contractor   string
	SectionIndex int // internal: index of the SUB TYPE section this permit belongs to
}

// sectionContact holds the raw applicant and contractor strings for one SUB TYPE section.
type sectionContact struct {
	applicant  string
	contractor string
}

// parsePermitLines converts a flat slice of text lines into permit records.
//
// Richmond PDFs are two-column tables. pdftotext (without -layout) reads the left column
// first (all permit records) then the right column (all APPLICANT/CONTRACTOR blocks), so
// contact info for a permit appears well after the permit's own lines. Each APPLICANT block
// corresponds to one SUB TYPE section, in the same order as those sections.
//
// This parser uses a two-phase approach:
//  1. Permit phase: parse permit fields by content type, tagging each record with its
//     section index (increments at each SUB TYPE header).
//  2. Contact phase: triggered by the first APPLICANT line on a page; collect one
//     applicant/contractor block per section in order.
//  3. Zip: after all lines are processed, associate contacts[sectionIndex] with each permit.
//
// Phase transitions:
//   - Permit phase to contact phase: first APPLICANT line encountered
//   - Contact phase to permit phase: SUB TYPE header encountered (next page beginning)
func parsePermitLines(lines []string) []permitRecord {
	var records []permitRecord
	var current *permitRecord
	var currentSubType string
	var nextLineIsAddress bool

	sectionIdx := -1                         // increments at each SUB TYPE header (permit phase)
	var contactIdx int                       // increments per saved contact block (contact phase)
	contacts := make(map[int]sectionContact) // keyed by contactIdx, which mirrors sectionIdx 0,1,2...
	var curContact sectionContact
	var inContacts bool // true = currently parsing right-column contact blocks
	var pendingApplicant bool
	var pendingContractor bool

	flush := func() {
		if current != nil {
			records = append(records, *current)
			current = nil
		}
	}

	saveContact := func() {
		contacts[contactIdx] = curContact
		contactIdx++
		curContact = sectionContact{}
		pendingApplicant = false
		pendingContractor = false
	}

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}

		// SUB TYPE header, handled in both phases.
		if m := subTypeRe.FindStringSubmatch(line); m != nil {
			if inContacts {
				saveContact()
				inContacts = false
			} else {
				flush()
			}
			sectionIdx++
			currentSubType = strings.TrimSpace(m[1])
			nextLineIsAddress = false
			continue
		}

		if m := applicantLineRe.FindStringSubmatch(line); m != nil {
			if !inContacts {
				flush()
				inContacts = true
			} else {
				saveContact()
			}
			if val := strings.TrimSpace(m[1]); val != "" {
				curContact.applicant = val
				pendingApplicant = false
			} else {
				pendingApplicant = true
			}
			pendingContractor = false
			continue
		}

		if m := contractorLineRe.FindStringSubmatch(line); m != nil {
			if inContacts {
				if val := strings.TrimSpace(m[1]); val != "" {
					curContact.contractor = val
					pendingContractor = false
				} else {
					pendingContractor = true
				}
				pendingApplicant = false
			}
			continue
		}

		if inContacts {
			if pendingApplicant {
				curContact.applicant = line
				pendingApplicant = false
			} else if pendingContractor {
				curContact.contractor = line
				pendingContractor = false
			}
			continue
		}

		if m := issueDateHeaderRe.FindStringSubmatch(line); m != nil {
			if current != nil {
				if t, err := time.Parse("2006/01/02", m[1]); err == nil {
					current.IssueDate = t
				}
			}
			continue
		}

		if skipLineRe.MatchString(line) {
			continue
		}

		if folderNumRe.MatchString(line) {
			flush()
			fn := line
			if m := folderNumExtractRe.FindString(line); m != "" {
				fn = m
			}
			current = &permitRecord{
				SubType:      currentSubType,
				FolderNumber: fn,
				SectionIndex: sectionIdx,
			}
			nextLineIsAddress = false
			continue
		}

		if current == nil {
			continue
		}

		if current.WorkProposed == "" && folderSuffixRe.MatchString(line) {
			current.FolderNumber = strings.TrimSpace(current.FolderNumber + " " + line)
			continue
		}

		if nextLineIsAddress {
			current.Address = line
			nextLineIsAddress = false
			continue
		}

		if m := folderNameDataRe.FindStringSubmatch(line); m != nil {
			current.Address = strings.TrimSpace(m[1])
			continue
		}

		if folderNameHeaderRe.MatchString(line) {
			nextLineIsAddress = true
			continue
		}

		dateToParse := line
		if m := issueDateWithCountRe.FindStringSubmatch(line); m != nil {
			dateToParse = m[1]
		}
		if issueDateRe.MatchString(dateToParse) {
			if t, err := time.Parse("2006/01/02", dateToParse); err == nil {
				current.IssueDate = t
			}
			continue
		}

		if valueRe.MatchString(line) {
			if current.ValueCAD == 0 {
				current.ValueCAD = parseDollarAmount(line)
			}
			continue
		}

		if permitCountRe.MatchString(line) {
			continue
		}

		switch {
		case current.WorkProposed == "":
			current.WorkProposed = line
		case current.Status == "":
			current.Status = line
		case current.Address == "":
			current.Address = line
		}
	}

	flush()
	if inContacts {
		saveContact()
	}

	for i := range records {
		if c, ok := contacts[records[i].SectionIndex]; ok {
			records[i].Applicant = c.applicant
			records[i].Contractor = c.contractor
		}
	}

	return records
}

// parseDollarAmount converts "$300,000.00" to 300000.
func parseDollarAmount(s string) int64 {
	s = strings.TrimPrefix(s, "$")
	s = strings.ReplaceAll(s, ",", "")
	if dot := strings.Index(s, "."); dot != -1 {
		s = s[:dot]
	}
	n, _ := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	return n
}
