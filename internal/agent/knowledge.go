package agent

import (
	"context"
	"net/http"
	"regexp"
	"strings"
)

const (
	maxPerSource = 4000
	maxTotal     = 14000
)

var (
	scriptRe  = regexp.MustCompile(`(?is)<script\b[^>]*>.*?</script>`)
	styleRe   = regexp.MustCompile(`(?is)<style\b[^>]*>.*?</style>`)
	allTagsRe = regexp.MustCompile(`(?s)<[^>]+>`)
	wsRe      = regexp.MustCompile(`\s+`)
)

// FetchLinks baixa cada URL, extrai o texto e compila a base de conhecimento.
func FetchLinks(ctx context.Context, urls []string) string {
	var parts []string
	total := 0

	for _, raw := range urls {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
			raw = "https://" + raw
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Notreve Chat Community)")
		req.Header.Set("Accept", "text/html")

		resp, err := blockedClient.Do(req)
		if err != nil {
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			resp.Body.Close()
			continue
		}

		body := make([]byte, 0, 512*1024)
		buf := make([]byte, 32*1024)
		for {
			n, err := resp.Body.Read(buf)
			body = append(body, buf[:n]...)
			if err != nil || n == 0 {
				break
			}
			if len(body) > 1024*1024 {
				break
			}
		}
		resp.Body.Close()

		html := string(body)
		html = scriptRe.ReplaceAllString(html, " ")
		html = styleRe.ReplaceAllString(html, " ")
		text := htmlUnescape(stripTags(html))
		text = wsRe.ReplaceAllString(text, " ")
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		if len(text) > maxPerSource {
			text = text[:maxPerSource]
		}

		part := "Fonte: " + raw + "\n" + text
		if total+len(part) > maxTotal {
			part = part[:maxTotal-total]
		}
		parts = append(parts, part)
		total += len(part)
		if total >= maxTotal {
			break
		}
	}

	return strings.Join(parts, "\n\n")
}

func stripTags(s string) string {
	return allTagsRe.ReplaceAllString(s, " ")
}

func htmlUnescape(s string) string {
	r := strings.NewReplacer(
		"&nbsp;", " ", "&amp;", "&", "&lt;", "<", "&gt;", ">",
		"&quot;", `"`, "&#39;", "'", "&apos;", "'", "&ndash;", "–",
		"&mdash;", "—", "&eacute;", "é", "&egrave;", "è", "&ecirc;", "ê",
		"&aacute;", "á", "&agrave;", "à", "&acirc;", "â", "&atilde;", "ã",
		"&ccedil;", "ç", "&oacute;", "ó", "&otilde;", "õ", "&uacute;", "ú",
		"&iacute;", "í", "&ntilde;", "ñ",
	)
	return r.Replace(s)
}
