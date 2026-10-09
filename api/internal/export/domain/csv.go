package domain

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strings"
)

// Header is the file's first line (ADR-0087, ADR-0092).
var Header = []string{"Date", "Amount", "Kind", "Category", "Description", "Note", "Source"}

// bom is the UTF-8 byte order mark. Excel reads a CSV file as UTF-8, Thai text included, only
// when the file starts with it; Google Sheets and other programs skip it.
const bom = "\ufeff"

// formulaStarts are the first characters that make a spreadsheet read a cell as a formula
// (OWASP, "CSV injection").
const formulaStarts = "=+-@\t\r"

// File is a CSV file of exported rows, built in memory (ADR-0092): UTF-8 with a byte order mark,
// lines ending in CRLF (RFC 4180), the header first, then one line per row. Fields with a comma,
// a double quote or a line break are quoted; a line break inside a field is written as CRLF too. The zero value is not usable; call NewFile.
type File struct {
	buf bytes.Buffer
	w   *csv.Writer
}

// NewFile returns a file holding the byte order mark and the header.
func NewFile() (*File, error) {
	f := &File{}
	f.buf.WriteString(bom)
	f.w = csv.NewWriter(&f.buf)
	f.w.UseCRLF = true
	if err := f.w.Write(Header); err != nil {
		return nil, fmt.Errorf("csv header: %w", err)
	}
	return f, nil
}

// Add writes rows in the order given. The date is YYYY-MM-DD and the amount plain decimal text
// (ADR-0092); the text the user typed (category, description, note) is made safe for
// spreadsheets with SafeCell.
func (f *File) Add(rows []Row) error {
	for _, r := range rows {
		rec := []string{
			r.OccurredOn.String(), r.Amount, r.Kind,
			SafeCell(r.Category), SafeCell(r.Description), SafeCell(r.Note), r.Source,
		}
		if err := f.w.Write(rec); err != nil {
			return fmt.Errorf("csv row: %w", err)
		}
	}
	return nil
}

// Bytes ends the file and returns its content.
func (f *File) Bytes() ([]byte, error) {
	f.w.Flush()
	if err := f.w.Error(); err != nil {
		return nil, fmt.Errorf("csv: %w", err)
	}
	return f.buf.Bytes(), nil
}

// SafeCell returns s so that a spreadsheet shows it as text: a value starting with =, +, -, @, a
// tab or a carriage return gets a leading apostrophe, which Excel, Google Sheets and LibreOffice
// do not run as a formula (OWASP, "CSV injection"). Other values are returned as they are.
func SafeCell(s string) string {
	if s != "" && strings.ContainsRune(formulaStarts, rune(s[0])) {
		return "'" + s
	}
	return s
}
