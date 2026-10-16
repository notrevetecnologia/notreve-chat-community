package channel

import "testing"

func TestNormalizeMediaMimeKeepsDeclared(t *testing.T) {
	if got := normalizeMediaMime("audio", "audio/ogg; codecs=opus", "", ""); got != "audio/ogg; codecs=opus" {
		t.Fatalf("mime declarado perdido: %q", got)
	}
}

func TestNormalizeMediaMimeInfersAudioFromLink(t *testing.T) {
	if got := normalizeMediaMime("audio", "", "", "https://cdn.example.com/abc.ogg"); got != "audio/ogg" {
		t.Fatalf("mime de áudio via mediaLink = %q, esperado audio/ogg", got)
	}
}

func TestNormalizeMediaMimeInfersFromFileName(t *testing.T) {
	if got := normalizeMediaMime("audio", "", "voice.ogg", ""); got != "audio/ogg" {
		t.Fatalf("mime por nome = %q, esperado audio/ogg", got)
	}
	if got := normalizeMediaMime("image", "", "foto.png", ""); got != "image/png" {
		t.Fatalf("mime por nome = %q, esperado image/png", got)
	}
	if got := normalizeMediaMime("document", "", "contrato.pdf", ""); got != "application/pdf" {
		t.Fatalf("mime por nome = %q, esperado application/pdf", got)
	}
}

func TestNormalizeMediaMimeReplacesOctetStream(t *testing.T) {
	if got := normalizeMediaMime("audio", "application/octet-stream", "", ""); got != "audio/ogg" {
		t.Fatalf("mime genérico não substituído: %q", got)
	}
}

func TestNormalizeMediaMimeStripsDataURI(t *testing.T) {
	if got := normalizeMediaMime("audio", "data:audio/ogg;base64,T2dn", "", ""); got != "audio/ogg" {
		t.Fatalf("mime de data URI = %q, esperado audio/ogg", got)
	}
}

func TestNormalizeNotreveEventCompactAudioKeepsBase64(t *testing.T) {
	payload := notrevePayload{MessageType: "audio", Base64: "T2dnUwACAAAA", MimeType: "audio/ogg", FileName: "voice.ogg"}
	var evt notreveEvent
	normalizeNotreveEvent(&evt, payload)
	md := firstMedia(&evt.Message)
	if md == nil {
		t.Fatal("áudio compacto não gerou mídia")
	}
	if md.Base64 != "T2dnUwACAAAA" || md.MimeType != "audio/ogg" || md.FileName != "voice.ogg" {
		t.Fatalf("base64/mime/name perdidos: %+v", md)
	}
}

func TestNormalizeNotreveEventCompactAudioSnakeCase(t *testing.T) {
	payload := notrevePayload{MessageTypeSnake: "audio", Base64: "T2dn", MimeTypeSnake: "audio/ogg", FileNameSnake: "voice.ogg"}
	normalizeNotrevePayload(&payload)
	var evt notreveEvent
	normalizeNotreveEvent(&evt, payload)
	md := firstMedia(&evt.Message)
	if md == nil || md.MimeType != "audio/ogg" || md.FileName != "voice.ogg" {
		t.Fatalf("snake_case de mídia não normalizado: %+v", md)
	}
}

func TestParseDataURIStripsBase64Prefix(t *testing.T) {
	data, mime := parseDataURI("data:audio/ogg;base64,T2dnUwACAAAA")
	if data != "T2dnUwACAAAA" || mime != "audio/ogg" {
		t.Fatalf("parseDataURI = (%q,%q)", data, mime)
	}
	data, mime = parseDataURI("T2dnUwACAAAA")
	if data != "T2dnUwACAAAA" || mime != "" {
		t.Fatalf("parseDataURI base64 cru = (%q,%q)", data, mime)
	}
}
