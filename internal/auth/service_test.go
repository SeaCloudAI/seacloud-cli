package auth

import "testing"

func TestBuildVerificationURLUsesAuthBaseOrigin(t *testing.T) {
	got := buildVerificationURL("https://real-cloud.seaart.ai/device", "ABCD-EFGH", "https://cloud.seaart.ai")
	want := "https://cloud.seaart.ai/device?code=ABCD-EFGH"
	if got != want {
		t.Fatalf("buildVerificationURL() = %q, want %q", got, want)
	}
}

func TestBuildVerificationURLFallsBackToServerURI(t *testing.T) {
	got := buildVerificationURL("https://cloud.seaart.ai/device", "ABCD-EFGH", "")
	want := "https://cloud.seaart.ai/device?code=ABCD-EFGH"
	if got != want {
		t.Fatalf("buildVerificationURL() = %q, want %q", got, want)
	}
}
