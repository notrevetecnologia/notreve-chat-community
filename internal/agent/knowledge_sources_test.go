package agent

import (
	"bytes"
	"compress/zlib"
	"testing"
)

func TestNormalizeKnowledgeTextTrimsAndLimitsRunes(t *testing.T) {
	got := NormalizeKnowledgeText("  olá\n\n mundo  ", 7)
	if got != "olá mun" {
		t.Fatalf("NormalizeKnowledgeText() = %q, want %q", got, "olá mun")
	}
}

func TestExtractPDFTextReadsLiteralAndHexStrings(t *testing.T) {
	raw := []byte("%PDF-1.4 1 0 obj << /Length 42 >> stream BT (Olá \\(mundo\\)) Tj <7465737465> Tj ET endstream endobj")
	got, err := ExtractPDFText(raw)
	if err != nil {
		t.Fatalf("ExtractPDFText() error = %v", err)
	}
	if got != "Olá (mundo) teste" {
		t.Fatalf("ExtractPDFText() = %q, want %q", got, "Olá (mundo) teste")
	}
}

func TestExtractPDFTextReadsFlateStream(t *testing.T) {
	var compressed bytes.Buffer
	writer := zlib.NewWriter(&compressed)
	if _, err := writer.Write([]byte("(conteúdo compactado)")); err != nil {
		t.Fatalf("zlib write error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("zlib close error = %v", err)
	}
	raw := append([]byte("%PDF-1.4 1 0 obj << /Filter /FlateDecode >> stream\n"), compressed.Bytes()...)
	raw = append(raw, []byte("\nendstream endobj")...)
	got, err := ExtractPDFText(raw)
	if err != nil {
		t.Fatalf("ExtractPDFText() error = %v", err)
	}
	if got != "conteúdo compactado" {
		t.Fatalf("ExtractPDFText() = %q, want %q", got, "conteúdo compactado")
	}
}

func TestExtractPDFTextRejectsInvalidAndScannedFiles(t *testing.T) {
	if _, err := ExtractPDFText([]byte("not a pdf")); err == nil {
		t.Fatal("ExtractPDFText() accepted an invalid PDF")
	}
	if _, err := ExtractPDFText([]byte("%PDF-1.4 /Type /Page")); err == nil {
		t.Fatal("ExtractPDFText() accepted a PDF without extractable text")
	}
}
