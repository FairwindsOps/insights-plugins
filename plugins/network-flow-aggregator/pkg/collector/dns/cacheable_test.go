package dns

import "testing"

func TestIsCacheableResponse(t *testing.T) {
	if !isCacheableResponse("A", "Success") {
		t.Fatal("expected A Success to be cacheable")
	}
	if !isCacheableResponse("AAAA", "success") {
		t.Fatal("expected AAAA success to be cacheable")
	}
	if isCacheableResponse("MX", "Success") {
		t.Fatal("expected MX to be skipped")
	}
	if isCacheableResponse("A", "NXDomain") {
		t.Fatal("expected NXDomain to be skipped")
	}
}
