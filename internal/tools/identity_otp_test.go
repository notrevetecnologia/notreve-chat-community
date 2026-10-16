package tools

import "testing"

func TestIdentityOTPCode(t *testing.T) {
	if got := identityOTPCode(map[string]interface{}{"_otp_code": "123456"}); got != "123456" {
		t.Fatalf("esperava o código OTP, obtive %q", got)
	}
	if got := identityOTPCode(map[string]interface{}{"_otp_code": 123456}); got != "" {
		t.Fatalf("código não-string deve resultar em vazio, obtive %q", got)
	}
	if got := identityOTPCode(nil); got != "" {
		t.Fatalf("sem args deve resultar em vazio")
	}
}

func TestWithoutReservedArgsStripsOTP(t *testing.T) {
	args := map[string]interface{}{"_identity": "123", "_challenge_answers": map[string]string{"x": "y"}, "_otp_code": "999", "cpf": "123"}
	out := withoutReservedArgs(args)
	if _, ok := out["_otp_code"]; ok {
		t.Fatal("_otp_code não deve ir para a requisição externa")
	}
	if out["cpf"] != "123" {
		t.Fatalf("argumentos normais devem permanecer, obtive %v", out)
	}
}
