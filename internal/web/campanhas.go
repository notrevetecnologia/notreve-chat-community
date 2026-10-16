package web

import "strings"

func mediaTypeFromName(name, mime string) string {
	n := strings.ToLower(name)
	switch {
	case strings.HasSuffix(n, ".jpg"), strings.HasSuffix(n, ".jpeg"), strings.HasSuffix(n, ".png"), strings.HasSuffix(n, ".gif"), strings.HasSuffix(n, ".webp"):
		return "image"
	case strings.HasSuffix(n, ".mp4"), strings.HasSuffix(n, ".mov"), strings.HasSuffix(n, ".mkv"), strings.HasSuffix(n, ".webm"):
		return "video"
	case strings.HasSuffix(n, ".mp3"), strings.HasSuffix(n, ".ogg"), strings.HasSuffix(n, ".wav"), strings.HasSuffix(n, ".m4a"):
		return "audio"
	case strings.HasSuffix(n, ".pdf"), strings.HasSuffix(n, ".doc"), strings.HasSuffix(n, ".docx"), strings.HasSuffix(n, ".xls"), strings.HasSuffix(n, ".xlsx"), strings.HasSuffix(n, ".txt"):
		return "document"
	}
	if strings.HasPrefix(mime, "image/") {
		return "image"
	}
	if strings.HasPrefix(mime, "video/") {
		return "video"
	}
	if strings.HasPrefix(mime, "audio/") {
		return "audio"
	}
	return "document"
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
