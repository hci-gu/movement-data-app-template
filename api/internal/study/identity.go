package study

import "regexp"

var verifiedIdentityFormat = regexp.MustCompile(`^[0-9]{12}$`)

// Gender reads the registered category from an already verified Swedish identity.
// Missing or malformed identities never acquire a gender-specific audience.
func Gender(personalNumber string) string {
	if !verifiedIdentityFormat.MatchString(personalNumber) {
		return "unknown"
	}
	if (personalNumber[10]-'0')%2 == 0 {
		return "female"
	}
	return "male"
}
