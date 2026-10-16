package llm

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGenerateImageParsesDataURI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("JSON inválido: %v", err)
		}
		modalities, _ := body["modalities"].([]interface{})
		if len(modalities) != 2 || modalities[0] != "image" || modalities[1] != "text" {
			t.Fatalf("modalities inesperadas: %v", body["modalities"])
		}
		config, _ := body["image_config"].(map[string]interface{})
		if config["aspect_ratio"] != "16:9" {
			t.Fatalf("aspect_ratio inesperado: %v", config["aspect_ratio"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"images":[{"url":"data:image/png;base64,QUJD"}]}}]}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "key", "image-model", 0)
	result, err := client.GenerateImage(context.Background(), "um gato", "16:9")
	if err != nil {
		t.Fatalf("GenerateImage retornou erro: %v", err)
	}
	if result.B64 != "QUJD" {
		t.Fatalf("base64 inesperado: %q", result.B64)
	}
	if result.MimeType != "image/png" {
		t.Fatalf("mime inesperado: %q", result.MimeType)
	}
}

func TestGenerateImageDownloadsURL(t *testing.T) {
	raw := []byte("imagem-bytes")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/img" {
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write(raw)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"images":[{"url":"` + "http://" + r.Host + `/img"}]}}]}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "key", "image-model", 0)
	result, err := client.GenerateImage(context.Background(), "paisagem", "")
	if err != nil {
		t.Fatalf("GenerateImage retornou erro: %v", err)
	}
	if result.B64 != base64.StdEncoding.EncodeToString(raw) {
		t.Fatalf("base64 inesperado: %q", result.B64)
	}
	if result.MimeType != "image/jpeg" {
		t.Fatalf("mime inesperado: %q", result.MimeType)
	}
}

func TestGenerateImageWithoutImageFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"sem imagem"}}]}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "key", "image-model", 0)
	_, err := client.GenerateImage(context.Background(), "algo", "")
	if err == nil || !strings.Contains(err.Error(), "não retornou imagem") {
		t.Fatalf("esperava erro de imagem ausente, obtive: %v", err)
	}
}

func TestGenerateImageRejectsEmptyPrompt(t *testing.T) {
	client := NewClient("http://example.invalid", "key", "image-model", 0)
	if _, err := client.GenerateImage(context.Background(), "  ", ""); err == nil {
		t.Fatal("esperava erro de prompt vazio")
	}
}
