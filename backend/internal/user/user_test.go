package user

import "testing"

func TestValidateEmail(t *testing.T) {
	valid := []string{
		"person@example.com",
		"first.last+tag@sub.domain.co",
		"a-b_c@example.io",
		"PESSOA@EXAMPLE.COM.BR",
		"01234@example.org",
	}
	for _, email := range valid {
		if !ValidateEmail(email) {
			t.Errorf("ValidateEmail(%q) = false, want true", email)
		}
	}

	invalid := []string{
		"",
		"  ",
		"not-an-email",
		"missing@tld",
		"@example.com",
		"person@example",
		"person@.com",
		"person@exam ple.com",
		"fail",
	}
	for _, email := range invalid {
		if ValidateEmail(email) {
			t.Errorf("ValidateEmail(%q) = true, want false", email)
		}
	}
}