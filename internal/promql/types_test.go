package promql

import (
	"strings"
	"testing"
)

func TestSampleUnmarshalAndParse(t *testing.T) {
	var s Sample
	if err := s.UnmarshalJSON([]byte(`[1700000000.5,"42.5"]`)); err != nil {
		t.Fatalf("UnmarshalJSON: %v", err)
	}
	if s.Time != 1700000000.5 {
		t.Errorf("time = %v", s.Time)
	}
	if s.Value != "42.5" {
		t.Errorf("value = %q", s.Value)
	}
	f, err := s.ParsedValue()
	if err != nil || f != 42.5 {
		t.Errorf("ParsedValue = %v, %v", f, err)
	}
}

func TestSampleUnmarshalRejectsBadShape(t *testing.T) {
	var s Sample
	if err := s.UnmarshalJSON([]byte(`[1700000000]`)); err == nil {
		t.Error("expected error for 1-element tuple")
	}
}

func TestErrorStrings(t *testing.T) {
	ae := &APIError{HTTPStatus: 400, ErrorType: "bad_data", Message: "nope"}
	if got := ae.Error(); !strings.Contains(got, "bad_data") || !strings.Contains(got, "nope") {
		t.Errorf("APIError.Error() = %q", got)
	}
	ce := &CredentialsError{err: errString("expired")}
	if got := ce.Error(); !strings.Contains(got, "expired") {
		t.Errorf("CredentialsError.Error() = %q", got)
	}
}

type errString string

func (e errString) Error() string { return string(e) }
