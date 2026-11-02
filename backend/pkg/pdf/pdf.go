// Package pdf writes minimal single-page text PDFs (invoices) without any
// third-party dependency. Text is encoded in WinAnsi (Latin-1 + €).
package pdf

import (
	"bytes"
	"fmt"
	"strings"
)

type Line struct {
	X, Y float64
	Size float64
	Bold bool
	Text string
}

// Render builds an A4 PDF containing the given lines.
func Render(lines []Line) []byte {
	var content bytes.Buffer
	for _, l := range lines {
		font := "F1"
		if l.Bold {
			font = "F2"
		}
		fmt.Fprintf(&content, "BT /%s %.1f Tf %.1f %.1f Td (%s) Tj ET\n", font, l.Size, l.X, l.Y, escape(winAnsi(l.Text)))
	}
	objs := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 4 0 R /F2 5 0 R >> >> /Contents 6 0 R >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold /Encoding /WinAnsiEncoding >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", content.Len(), content.String()),
	}
	var out bytes.Buffer
	out.WriteString("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")
	offsets := make([]int, len(objs))
	for i, o := range objs {
		offsets[i] = out.Len()
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(objs)+1)
	for _, off := range offsets {
		fmt.Fprintf(&out, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objs)+1, xref)
	return out.Bytes()
}

func winAnsi(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '€':
			b.WriteByte(0x80)
		case r == '’':
			b.WriteByte(0x92)
		case r == '–':
			b.WriteByte(0x96)
		case r < 256:
			b.WriteByte(byte(r))
		default:
			b.WriteByte('?')
		}
	}
	return b.String()
}

func escape(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `(`, `\(`, `)`, `\)`, "\r", " ", "\n", " ")
	return r.Replace(s)
}
