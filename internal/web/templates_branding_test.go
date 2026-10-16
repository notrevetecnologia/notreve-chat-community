package web

import (
	"bytes"
	"html/template"
	"path/filepath"
	"testing"
)

func TestLoadTemplatesParses(t *testing.T) {
	webroot := filepath.Join("..", "..", "web")
	s := &Server{webroot: webroot}
	if err := s.loadTemplates(); err != nil {
		t.Fatalf("falha ao parsear templates: %v", err)
	}
	loginTpl, err := template.ParseFiles(filepath.Join(webroot, "templates", "login.html"))
	if err != nil {
		t.Fatalf("falha ao parsear login.html: %v", err)
	}
	var buf bytes.Buffer
	if err := loginTpl.ExecuteTemplate(&buf, "login.html", map[string]interface{}{"Branding": reactBrandingDefaults}); err != nil {
		t.Fatalf("falha ao renderizar login.html: %v", err)
	}
	if got := buf.String(); got == "" {
		t.Fatal("login.html renderizado vazio")
	}
}
