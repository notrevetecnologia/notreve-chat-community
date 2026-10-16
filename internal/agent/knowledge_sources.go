package agent

import (
	"bytes"
	"compress/zlib"
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/notrevetecnologia/notreve-chat-community/internal/llm"
)

const (
	MaxKnowledgeTextChars   = 100000
	MaxKnowledgePDFBytes    = 10 << 20
	MaxKnowledgeAudioBytes  = 25 << 20
	MaxKnowledgeSourceChars = 500000
)

var knowledgeWhitespace = regexp.MustCompile(`\s+`)

func NormalizeKnowledgeText(value string, limit int) string {
	value = strings.TrimSpace(knowledgeWhitespace.ReplaceAllString(value, " "))
	if limit > 0 && len([]rune(value)) > limit {
		return string([]rune(value)[:limit])
	}
	return value
}

func TranscribeKnowledgeAudio(ctx context.Context, baseURL, apiKey, model string, raw []byte, mimeType, fileName string) (string, error) {
	if strings.TrimSpace(baseURL) == "" || strings.TrimSpace(apiKey) == "" {
		return "", fmt.Errorf("STT não configurado para treinamento de áudio")
	}
	if len(raw) == 0 || len(raw) > MaxKnowledgeAudioBytes {
		return "", fmt.Errorf("áudio deve ter entre 1 byte e %d MB", MaxKnowledgeAudioBytes/(1<<20))
	}
	if strings.TrimSpace(model) == "" {
		model = "whisper-1"
	}
	if strings.TrimSpace(fileName) == "" {
		fileName = "audio.bin"
	}
	client := llm.NewClient(baseURL, apiKey, model, 0)
	text, err := client.Transcribe(ctx, baseURL, apiKey, model, raw, mimeType, fileName)
	if err != nil {
		return "", err
	}
	text = NormalizeKnowledgeText(text, MaxKnowledgeTextChars)
	if text == "" {
		return "", fmt.Errorf("a transcrição não retornou texto")
	}
	return text, nil
}

func ExtractPDFText(raw []byte) (string, error) {
	if len(raw) < 5 || string(raw[:5]) != "%PDF-" {
		return "", fmt.Errorf("arquivo PDF inválido")
	}
	if len(raw) > MaxKnowledgePDFBytes {
		return "", fmt.Errorf("PDF excede o limite de %d MB", MaxKnowledgePDFBytes/(1<<20))
	}
	parts := make([]string, 0, 4)
	parts = append(parts, extractPDFStringsOutsideStreams(raw))
	for offset := 0; ; {
		start := bytes.Index(raw[offset:], []byte("stream"))
		if start < 0 {
			break
		}
		start += offset + len("stream")
		if start < len(raw) && raw[start] == '\r' {
			start++
		}
		if start < len(raw) && raw[start] == '\n' {
			start++
		}
		end := bytes.Index(raw[start:], []byte("endstream"))
		if end < 0 {
			break
		}
		end += start
		windowStart := start - 256
		if windowStart < 0 {
			windowStart = 0
		}
		if bytes.Contains(raw[windowStart:start], []byte("/FlateDecode")) {
			reader, err := zlib.NewReader(bytes.NewReader(raw[start:end]))
			if err == nil {
				decoded, readErr := io.ReadAll(io.LimitReader(reader, MaxKnowledgePDFBytes))
				reader.Close()
				if readErr == nil {
					parts = append(parts, extractPDFStrings(decoded))
				}
			}
		} else {
			parts = append(parts, extractPDFStrings(raw[start:end]))
		}
		offset = end + len("endstream")
	}
	text := NormalizeKnowledgeText(strings.Join(parts, " "), MaxKnowledgeTextChars)
	if text == "" {
		return "", fmt.Errorf("PDF não contém texto extraível; arquivos escaneados precisam de OCR")
	}
	return text, nil
}

func extractPDFStrings(raw []byte) string {
	var out strings.Builder
	for i := 0; i < len(raw); i++ {
		switch raw[i] {
		case '(':
			value, next := readPDFLiteral(raw, i)
			if len(value) > 0 {
				out.WriteString(value)
				out.WriteByte(' ')
			}
			i = next
		case '<':
			if i > 0 && raw[i-1] == '<' {
				continue
			}
			if i+1 < len(raw) && raw[i+1] != '<' {
				value, next := readPDFHex(raw, i)
				if len(value) > 0 {
					out.WriteString(value)
					out.WriteByte(' ')
				}
				i = next
			}
		}
	}
	return out.String()
}

func extractPDFStringsOutsideStreams(raw []byte) string {
	clean := append([]byte(nil), raw...)
	for offset := 0; offset < len(clean); {
		start := bytes.Index(clean[offset:], []byte("stream"))
		if start < 0 {
			break
		}
		start += offset + len("stream")
		if start < len(clean) && clean[start] == '\r' {
			start++
		}
		if start < len(clean) && clean[start] == '\n' {
			start++
		}
		end := bytes.Index(clean[start:], []byte("endstream"))
		if end < 0 {
			break
		}
		end += start
		for i := start; i < end; i++ {
			clean[i] = ' '
		}
		offset = end + len("endstream")
	}
	return extractPDFStrings(clean)
}

func readPDFLiteral(raw []byte, start int) (string, int) {
	var out strings.Builder
	depth := 0
	for i := start; i < len(raw); i++ {
		char := raw[i]
		if char == '(' && (i == start || raw[i-1] != '\\') {
			depth++
			if depth > 1 {
				out.WriteByte(char)
			}
			continue
		}
		if char == ')' && (i == start || raw[i-1] != '\\') {
			depth--
			if depth == 0 {
				return out.String(), i
			}
			out.WriteByte(char)
			continue
		}
		if char == '\\' && i+1 < len(raw) {
			i++
			switch raw[i] {
			case 'n':
				out.WriteByte('\n')
			case 'r':
				out.WriteByte('\r')
			case 't':
				out.WriteByte('\t')
			case 'b':
				out.WriteByte('\b')
			case 'f':
				out.WriteByte('\f')
			case '\\', '(', ')':
				out.WriteByte(raw[i])
			default:
				out.WriteByte(raw[i])
			}
			continue
		}
		if depth > 0 {
			out.WriteByte(char)
		}
	}
	return out.String(), len(raw)
}

func readPDFHex(raw []byte, start int) (string, int) {
	hex := make([]byte, 0, 64)
	for i := start + 1; i < len(raw); i++ {
		if raw[i] == '>' {
			if len(hex)%2 != 0 {
				hex = append(hex, '0')
			}
			decoded := make([]byte, len(hex)/2)
			for j := range decoded {
				high, highOK := fromHex(hex[j*2])
				low, lowOK := fromHex(hex[j*2+1])
				if !highOK || !lowOK {
					return "", i
				}
				decoded[j] = high<<4 | low
			}
			return string(decoded), i
		}
		if raw[i] != ' ' && raw[i] != '\n' && raw[i] != '\r' && raw[i] != '\t' {
			if _, ok := fromHex(raw[i]); !ok {
				return "", i
			}
			hex = append(hex, raw[i])
		}
	}
	return "", len(raw)
}

func fromHex(value byte) (byte, bool) {
	switch {
	case value >= '0' && value <= '9':
		return value - '0', true
	case value >= 'a' && value <= 'f':
		return value - 'a' + 10, true
	case value >= 'A' && value <= 'F':
		return value - 'A' + 10, true
	default:
		return 0, false
	}
}
