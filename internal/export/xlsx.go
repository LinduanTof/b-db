package export

import (
	"archive/zip"
	"bufio"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/LinduanTof/dbrix/internal/db"
)

type Sheet struct {
	Name    string
	Columns []string
	Rows    [][]any
}

// XLSX writes a minimal Office Open XML workbook with one sheet per entry.
func XLSX(w io.Writer, sheets []Sheet) error {
	if len(sheets) == 0 {
		sheets = []Sheet{{Name: "Sheet1"}}
	}
	names := sheetNames(sheets)

	zw := zip.NewWriter(w)
	write := func(name, content string) error {
		file, err := zw.Create(name)
		if err != nil {
			return err
		}
		_, err = io.WriteString(file, content)
		return err
	}

	if err := write("[Content_Types].xml", contentTypes(len(names))); err != nil {
		return err
	}
	if err := write("_rels/.rels", rootRels()); err != nil {
		return err
	}
	if err := write("xl/workbook.xml", workbook(names)); err != nil {
		return err
	}
	if err := write("xl/_rels/workbook.xml.rels", workbookRels(len(names))); err != nil {
		return err
	}
	for i, sheet := range sheets {
		path := fmt.Sprintf("xl/worksheets/sheet%d.xml", i+1)
		if err := write(path, worksheet(sheet)); err != nil {
			return err
		}
	}
	return zw.Close()
}

const xmlHeader = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`

const nsMain = "http://schemas.openxmlformats.org/spreadsheetml/2006/main"
const nsRels = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"

func contentTypes(sheets int) string {
	var b strings.Builder
	b.WriteString(xmlHeader)
	b.WriteString(`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">`)
	b.WriteString(`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>`)
	b.WriteString(`<Default Extension="xml" ContentType="application/xml"/>`)
	b.WriteString(`<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>`)
	for i := 0; i < sheets; i++ {
		fmt.Fprintf(&b, `<Override PartName="/xl/worksheets/sheet%d.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>`, i+1)
	}
	b.WriteString(`</Types>`)
	return b.String()
}

func rootRels() string {
	return xmlHeader + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="` + nsRels + `/officeDocument" Target="xl/workbook.xml"/>` +
		`</Relationships>`
}

func workbook(names []string) string {
	var b strings.Builder
	b.WriteString(xmlHeader)
	b.WriteString(`<workbook xmlns="` + nsMain + `" xmlns:r="` + nsRels + `"><sheets>`)
	for i, name := range names {
		fmt.Fprintf(&b, `<sheet name="%s" sheetId="%d" r:id="rId%d"/>`, xmlEscape(name), i+1, i+1)
	}
	b.WriteString(`</sheets></workbook>`)
	return b.String()
}

func workbookRels(sheets int) string {
	var b strings.Builder
	b.WriteString(xmlHeader)
	b.WriteString(`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	for i := 0; i < sheets; i++ {
		fmt.Fprintf(&b, `<Relationship Id="rId%d" Type="`+nsRels+`/worksheet" Target="worksheets/sheet%d.xml"/>`, i+1, i+1)
	}
	b.WriteString(`</Relationships>`)
	return b.String()
}

func worksheet(sheet Sheet) string {
	var b strings.Builder
	b.WriteString(xmlHeader)
	b.WriteString(`<worksheet xmlns="` + nsMain + `"><sheetData>`)
	rowIndex := 0
	if len(sheet.Columns) > 0 {
		b.WriteString(`<row r="1">`)
		for col, name := range sheet.Columns {
			writeCell(&b, col, 0, name)
		}
		b.WriteString(`</row>`)
		rowIndex = 1
	}
	for _, row := range sheet.Rows {
		b.WriteString(`<row r="` + strconv.Itoa(rowIndex+1) + `">`)
		for col := 0; col < len(sheet.Columns); col++ {
			if col < len(row) {
				writeCell(&b, col, rowIndex, row[col])
			}
		}
		b.WriteString(`</row>`)
		rowIndex++
	}
	b.WriteString(`</sheetData></worksheet>`)
	return b.String()
}

func writeCell(b *strings.Builder, col, row int, value any) {
	b.WriteString(cellXML(col, row, value))
}

func cellXML(col, row int, value any) string {
	text, number := sheetValue(value)
	if !number && text == "" {
		return ""
	}
	ref := cellRef(col, row)
	if number {
		return `<c r="` + ref + `"><v>` + text + `</v></c>`
	}
	return `<c r="` + ref + `" t="inlineStr"><is><t xml:space="preserve">` + xmlEscape(cleanCellText(text)) + `</t></is></c>`
}

func cleanCellText(value string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return ' '
		}
		return r
	}, value)
}

func sheetValue(value any) (string, bool) {
	switch x := value.(type) {
	case nil:
		return "", false
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprint(x), true
	case float32:
		return strconv.FormatFloat(float64(x), 'g', -1, 32), true
	case float64:
		return strconv.FormatFloat(x, 'g', -1, 64), true
	case bool:
		if x {
			return "TRUE", false
		}
		return "FALSE", false
	case []byte:
		return string(x), false
	default:
		return fmt.Sprint(x), false
	}
}

func cellRef(col, row int) string {
	var letters []byte
	for col >= 0 {
		letters = append([]byte{byte('A' + col%26)}, letters...)
		col = col/26 - 1
	}
	return string(letters) + strconv.Itoa(row+1)
}

func xmlEscape(value string) string {
	var b bytes.Buffer
	if err := xml.EscapeText(&b, []byte(value)); err != nil {
		return value
	}
	return b.String()
}

func sheetNames(sheets []Sheet) []string {
	names := make([]string, len(sheets))
	for i, sheet := range sheets {
		names[i] = sheet.Name
	}
	return uniqueSheetNames(names)
}

func uniqueSheetNames(names []string) []string {
	used := map[string]bool{}
	out := make([]string, len(names))
	for i, raw := range names {
		name := strings.TrimSpace(raw)
		name = strings.Map(func(r rune) rune {
			switch r {
			case '[', ']', ':', '*', '?', '/', '\\':
				return '_'
			}
			return r
		}, name)
		runes := []rune(name)
		if len(runes) > 31 {
			name = string(runes[:31])
		}
		if name == "" {
			name = fmt.Sprintf("Sheet%d", i+1)
		}
		base := name
		for n := 2; used[name]; n++ {
			suffix := fmt.Sprintf(" (%d)", n)
			trimmed := []rune(base)
			if limit := 31 - len([]rune(suffix)); len(trimmed) > limit {
				trimmed = trimmed[:limit]
			}
			name = string(trimmed) + suffix
		}
		used[name] = true
		out[i] = name
	}
	return out
}

// SheetSource lazily opens a row cursor for one worksheet.
type SheetSource struct {
	Name string
	Open func() (db.RowStream, error)
	Done func()
}

const maxXLSXRows = 1048575

// XLSXStream writes sheets as their cursors are read, keeping memory bounded.
func XLSXStream(w io.Writer, sheets []SheetSource) error {
	names := make([]string, len(sheets))
	for i, sheet := range sheets {
		names[i] = sheet.Name
	}
	names = uniqueSheetNames(names)

	zw := zip.NewWriter(w)
	write := func(name, content string) error {
		file, err := zw.Create(name)
		if err != nil {
			return err
		}
		_, err = io.WriteString(file, content)
		return err
	}
	if err := write("[Content_Types].xml", contentTypes(len(names))); err != nil {
		return err
	}
	if err := write("_rels/.rels", rootRels()); err != nil {
		return err
	}
	if err := write("xl/workbook.xml", workbook(names)); err != nil {
		return err
	}
	if err := write("xl/_rels/workbook.xml.rels", workbookRels(len(names))); err != nil {
		return err
	}
	for i, sheet := range sheets {
		stream, err := sheet.Open()
		if err != nil {
			return err
		}
		file, err := zw.Create(fmt.Sprintf("xl/worksheets/sheet%d.xml", i+1))
		if err != nil {
			stream.Close()
			return err
		}
		err = writeWorksheetStream(file, stream)
		if closeErr := stream.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			return err
		}
		if sheet.Done != nil {
			sheet.Done()
		}
	}
	return zw.Close()
}

func writeWorksheetStream(w io.Writer, stream db.RowStream) error {
	bw := bufio.NewWriterSize(w, 64*1024)
	bw.WriteString(xmlHeader)
	bw.WriteString(`<worksheet xmlns="` + nsMain + `"><sheetData>`)
	columns := stream.Columns()
	row := 0
	if len(columns) > 0 {
		bw.WriteString(`<row r="1">`)
		for col, column := range columns {
			bw.WriteString(cellXML(col, 0, column.Name))
		}
		bw.WriteString(`</row>`)
		row = 1
	}
	for stream.Next() {
		if row >= maxXLSXRows {
			return fmt.Errorf("xlsx supports at most %d rows; export CSV or SQL instead", maxXLSXRows)
		}
		values, err := stream.Values()
		if err != nil {
			return err
		}
		bw.WriteString(`<row r="` + strconv.Itoa(row+1) + `">`)
		for col := 0; col < len(columns); col++ {
			if col < len(values) {
				bw.WriteString(cellXML(col, row, values[col]))
			}
		}
		bw.WriteString(`</row>`)
		row++
	}
	if err := stream.Err(); err != nil {
		return err
	}
	bw.WriteString(`</sheetData></worksheet>`)
	return bw.Flush()
}
