// This file holds the one integer syntax Redis accepts, shared by stored
// values and command arguments so both reject the same inputs. It must not
// know where the string came from.
package store

import "strconv"

// ParseInteger accepts only the canonical base-10 form of an int64. Redis
// rejects "+5", "05" and " 5" even though strconv.ParseInt takes some of them,
// so the round trip through FormatInt is the check.
func ParseInteger(text string) (int64, bool) {
	number, err := strconv.ParseInt(text, 10, 64)
	if err != nil || strconv.FormatInt(number, 10) != text {
		return 0, false
	}
	return number, true
}
