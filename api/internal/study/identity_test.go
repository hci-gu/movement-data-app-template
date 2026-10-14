package study

import "testing"

func TestGender(t *testing.T) {
	for input, want := range map[string]string{"200001010024": "female", "200001010032": "male", "": "unknown", "20000101-0024": "unknown", "notanidentity": "unknown"} {
		if got := Gender(input); got != want {
			t.Errorf("Gender(%q) = %q; want %q", input, got, want)
		}
	}
}
