package systemtest

import (
	"testing"
	"time"
)

func TestTOTPMatchesRFC6238(t *testing.T) {
	const secret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"

	for _, tc := range []struct {
		unix int64
		code string
	}{
		{59, "287082"},
		{1111111109, "081804"},
		{1111111111, "050471"},
		{1234567890, "005924"},
		{2000000000, "279037"},
		{20000000000, "353130"},
	} {
		code, err := TOTP(secret, time.Unix(tc.unix, 0))
		if err != nil || code != tc.code {
			t.Errorf("%d: got %q, %v, want %q", tc.unix, code, err, tc.code)
		}
	}
}

func TestTOTPAcceptsLowercaseSpacesAndPadding(t *testing.T) {
	code, err := TOTP("gezd gnbv gy3t qojq gezd gnbv gy3t qojq====", time.Unix(59, 0))
	if err != nil || code != "287082" {
		t.Errorf("got %q, %v", code, err)
	}
}

func TestTOTPRejectsAnInvalidSecret(t *testing.T) {
	if _, err := TOTP("not base32!", time.Now()); err == nil {
		t.Error("expected an error")
	}
}
