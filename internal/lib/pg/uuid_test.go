package pg

import "testing"

func TestIsUUID(t *testing.T) {
	cases := map[string]bool{
		"11111111-1111-4111-8111-111111111111":   true,
		"AAAAAAAA-aaaa-4AAA-8aaa-aaaaaaaaaaaa":   true,
		"":                                       false,
		"nope":                                   false,
		"11111111111141118111111111111111":       false,
		"{11111111-1111-4111-8111-111111111111}": false,
		"urn:uuid:11111111-1111-4111-8111-111111111111": false,
		"11111111-1111-4111-8111-11111111111g":          false,
	}
	for in, want := range cases {
		if got := IsUUID(in); got != want {
			t.Errorf("IsUUID(%q) = %v, want %v", in, got, want)
		}
	}
}
