package web

import (
	"strings"
	"testing"
)

func TestMFATemplateRendersQRDataURI(t *testing.T) {
	var output strings.Builder
	data := mfaPageData{
		Title:     "Ative o MFA",
		BrandName: "Notreve Chat Community",
		Secret:    "JBSWY3DPEHPK3PXP",
		QR:        totpQRDataURI("otpauth://totp/Notreve:super_admin?secret=JBSWY3DPEHPK3PXP&issuer=Notreve"),
	}
	if err := mfaPageTemplate.Execute(&output, data); err != nil {
		t.Fatalf("executar template MFA: %v", err)
	}
	rendered := output.String()
	if strings.Contains(rendered, "#ZgotmplZ") {
		t.Fatal("template sanitizou a URI do QR Code")
	}
	if !strings.Contains(rendered, "src=\"data:image/png;base64,") {
		t.Fatal("template não renderizou a URI do QR Code")
	}
}
