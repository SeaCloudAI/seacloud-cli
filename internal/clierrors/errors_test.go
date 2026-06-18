package clierrors

import (
	"errors"
	"strings"
	"testing"
)

func TestIsInsufficientBalanceRecognizesBalanceErrorCodes(t *testing.T) {
	err := NewAPIError(402, []byte(`{"status":{"code":402,"message":"Insufficient credits","error_code":40201}}`))
	if !IsInsufficientBalance(err) {
		t.Fatalf("expected insufficient balance classification for %#v", err)
	}
}

func TestIsInsufficientBalanceRejectsGenericPaymentRequired(t *testing.T) {
	err := NewAPIError(402, []byte(`{"status":{"code":402,"message":"payment required","error_code":40200}}`))
	if IsInsufficientBalance(err) {
		t.Fatalf("did not expect insufficient balance classification for %#v", err)
	}
}

func TestIsInsufficientBalanceHandlesNil(t *testing.T) {
	if IsInsufficientBalance(nil) {
		t.Fatal("nil error should not be classified as insufficient balance")
	}
}

func TestErrSubmitFailedUsesBalanceHint(t *testing.T) {
	err := ErrSubmitFailed(NewAPIError(402, []byte(`{"status":{"code":402,"message":"Insufficient credits","error_code":40201}}`)))
	got := err.Error()
	if !strings.Contains(got, "insufficient balance") ||
		!strings.Contains(got, "seacloud account balance") ||
		!strings.Contains(got, "https://cloud.seaart.ai/settings/credits") ||
		strings.Contains(got, "auth status") {
		t.Fatalf("unexpected submit error: %q", got)
	}
}

func TestErrSubmitFailedUsesGenerationEndpointHint(t *testing.T) {
	err := ErrSubmitFailed(errors.New("generation base URL not configured: set SEACLOUD_GENERATION_URL"))
	got := err.Error()
	if !strings.Contains(got, "generation base URL not configured") ||
		!strings.Contains(got, "SEACLOUD_GENERATION_URL=https://cloud.seaart.ai") ||
		strings.Contains(got, "auth status") {
		t.Fatalf("unexpected submit error: %q", got)
	}
}
