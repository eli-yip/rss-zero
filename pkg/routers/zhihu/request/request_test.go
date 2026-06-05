package request

import "testing"

func TestIsAccountDestroyedResp(t *testing.T) {
	body := []byte(`{"error":{"message":"该账号已注销","code":310000,"name":"AccountDestroyError"}}`)

	if !isAccountDestroyedResp(body) {
		t.Fatal("expected account destroyed response to be detected")
	}
}

func TestIsAccountDestroyedRespReturnsFalseForOtherErrors(t *testing.T) {
	body := []byte(`{"error":{"message":"forbidden","code":403,"name":"ForbiddenError"}}`)

	if isAccountDestroyedResp(body) {
		t.Fatal("expected non-account destroyed response to be ignored")
	}
}
