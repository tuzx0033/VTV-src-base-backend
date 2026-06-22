// Package xexcel: thin helpers over github.com/xuri/excelize/v2 for importing
// xlsx files (generic sheet imports — see
// ../erp/docs/parity/07-import-excel.md). The pure parsing helpers here are
// kept separate from use cases (file → []Row → use case maps Row → domain).
package xexcel

import (
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"github.com/xuri/excelize/v2"
)

// OpenFile opens an xlsx file.
func OpenFile(path string) (*excelize.File, error) { return excelize.OpenFile(path) }

// FirstSheet returns the name of the first (active) sheet.
func FirstSheet(f *excelize.File) string {
	if s := f.GetSheetName(f.GetActiveSheetIndex()); s != "" {
		return s
	}
	if names := f.GetSheetList(); len(names) > 0 {
		return names[0]
	}
	return "Sheet1"
}

// Row is one data row keyed by NORMALIZED header (see NormalizeHeader).
type Row map[string]string

// ReadSheetAsMaps reads a sheet where the first non-empty row is the header.
// Returns rows keyed by normalized header. headerRowIdx is 0-based.
func ReadSheetAsMaps(f *excelize.File, sheet string, headerRowIdx int) ([]Row, []string, error) {
	raw, err := f.GetRows(sheet)
	if err != nil {
		return nil, nil, err
	}
	if headerRowIdx >= len(raw) {
		return nil, nil, nil
	}
	headers := make([]string, len(raw[headerRowIdx]))
	for i, h := range raw[headerRowIdx] {
		headers[i] = NormalizeHeader(h)
	}
	out := make([]Row, 0, len(raw)-headerRowIdx-1)
	for r := headerRowIdx + 1; r < len(raw); r++ {
		cells := raw[r]
		row := make(Row, len(headers))
		empty := true
		for i, h := range headers {
			if h == "" {
				continue
			}
			v := ""
			if i < len(cells) {
				v = strings.TrimSpace(cells[i])
			}
			if v != "" {
				empty = false
			}
			row[h] = v
		}
		if empty {
			continue
		}
		out = append(out, row)
	}
	return out, headers, nil
}

// NormalizeHeader lowercases, trims, strips Vietnamese diacritics, and collapses
// whitespace — so "Ngày air video" and "ngay  air video " both → "ngay air video".
func NormalizeHeader(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = StripDiacritics(s)
	return strings.Join(strings.Fields(s), " ")
}

// StripDiacritics maps Vietnamese accented letters to their ASCII base form.
func StripDiacritics(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if repl, ok := vnDiacritics[r]; ok {
			b.WriteRune(repl)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

var vnDiacritics = func() map[rune]rune {
	m := map[rune]rune{}
	groups := map[rune]string{
		'a': "àáảãạăằắẳẵặâầấẩẫậ", 'e': "èéẻẽẹêềếểễệ", 'i': "ìíỉĩị",
		'o': "òóỏõọôồốổỗộơờớởỡợ", 'u': "ùúủũụưừứửữự", 'y': "ỳýỷỹỵ", 'd': "đ",
	}
	for base, accented := range groups {
		for _, r := range accented {
			m[r] = base
			// also uppercase variants
			ub := []rune(strings.ToUpper(string(base)))[0]
			for _, ur := range strings.ToUpper(string(r)) {
				m[ur] = ub
			}
		}
	}
	return m
}()

// Pick returns the first non-empty value among the given (already-normalized)
// header aliases. Use it to tolerate header variations in source files.
func (r Row) Pick(aliases ...string) string {
	for _, a := range aliases {
		if v, ok := r[NormalizeHeader(a)]; ok && v != "" {
			return v
		}
	}
	return ""
}

// Has reports whether any of the aliases has a non-empty value.
func (r Row) Has(aliases ...string) bool { return r.Pick(aliases...) != "" }

// ── value parsing (tolerant of "1.234,56" / "1,234.56" / "đ" / "%" etc.) ──────

// ParseInt parses an integer from a messy cell ("1.234" / "1,234" / "" → 0).
func ParseInt(s string) (int64, bool) {
	c := cleanNumber(s)
	if c == "" {
		return 0, false
	}
	if i, err := strconv.ParseInt(c, 10, 64); err == nil {
		return i, true
	}
	// fall back via float (e.g. "12.0")
	if f, err := strconv.ParseFloat(c, 64); err == nil {
		return int64(f), true
	}
	return 0, false
}

// ParseDecimal parses a money/decimal value from a messy cell.
func ParseDecimal(s string) (decimal.Decimal, bool) {
	c := cleanNumber(s)
	if c == "" {
		return decimal.Zero, false
	}
	d, err := decimal.NewFromString(c)
	if err != nil {
		return decimal.Zero, false
	}
	return d, true
}

// ParseDate tries a list of common layouts (incl. the US "MM/DD/YYYY" layout).
func ParseDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	layouts := []string{
		"2006-01-02", "2006-01-02 15:04:05", "2006-01-02T15:04:05Z07:00",
		"01/02/2006", "01/02/2006 15:04:05", "1/2/2006",
		"02/01/2006", "2/1/2006",
		"02-01-2006", "2006/01/02",
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t.UTC(), true
		}
	}
	// excelize sometimes returns serial numbers as strings
	if f, err := strconv.ParseFloat(s, 64); err == nil && f > 1 {
		// Excel epoch 1899-12-30
		return time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC).Add(time.Duration(f * 24 * float64(time.Hour))), true
	}
	return time.Time{}, false
}

// cleanNumber strips currency symbols, %, spaces, and normalizes the decimal sep.
func cleanNumber(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	// drop everything except digits, separators and sign
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' || r == '.' || r == ',' || r == '-' || r == '+' {
			b.WriteRune(r)
		}
	}
	c := b.String()
	if c == "" {
		return ""
	}
	lastDot := strings.LastIndex(c, ".")
	lastComma := strings.LastIndex(c, ",")
	switch {
	case lastDot >= 0 && lastComma >= 0:
		if lastComma > lastDot { // "1.234,56" → comma is decimal sep
			c = strings.ReplaceAll(c, ".", "")
			c = strings.Replace(c, ",", ".", 1)
		} else { // "1,234.56" → comma is thousands sep
			c = strings.ReplaceAll(c, ",", "")
		}
	case lastComma >= 0: // only commas
		// if it looks like a thousands group ("1,234") drop them, else treat as decimal
		if len(c)-lastComma-1 == 3 && strings.Count(c, ",") >= 1 && !strings.Contains(c[:lastComma], ".") {
			c = strings.ReplaceAll(c, ",", "")
		} else {
			c = strings.Replace(c, ",", ".", 1)
			c = strings.ReplaceAll(c, ",", "")
		}
	default: // only dots or none — assume dot is decimal sep, but strip extra dots used as thousands
		if strings.Count(c, ".") > 1 {
			i := strings.LastIndex(c, ".")
			c = strings.ReplaceAll(c[:i], ".", "") + c[i:]
		}
	}
	return c
}
