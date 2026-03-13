// This file implements the glob dialect KEYS uses. path.Match is close but
// wrong for Redis: its '*' and '?' refuse to match '/', and it rejects a
// malformed pattern where Redis quietly matches it literally. It must stay a
// pure function of pattern and key.
package store

// matchGlob follows Redis's stringmatchlen: '*' matches any run of bytes, '?'
// one byte, "[...]" a set with optional '^' negation and "a-z" ranges, and '\'
// escapes the next byte. Matching is byte-wise, not rune-wise, as in Redis.
func matchGlob(pattern, key string) bool {
	for len(pattern) > 0 {
		switch pattern[0] {
		case '*':
			for len(pattern) > 1 && pattern[1] == '*' {
				pattern = pattern[1:]
			}
			if len(pattern) == 1 {
				return true
			}
			for start := 0; start <= len(key); start++ {
				if matchGlob(pattern[1:], key[start:]) {
					return true
				}
			}
			return false
		case '?':
			if len(key) == 0 {
				return false
			}
			pattern, key = pattern[1:], key[1:]
		case '[':
			if len(key) == 0 {
				return false
			}
			matched, rest := matchByteSet(pattern[1:], key[0])
			if !matched {
				return false
			}
			pattern, key = rest, key[1:]
		default:
			if pattern[0] == '\\' && len(pattern) > 1 {
				pattern = pattern[1:]
			}
			if len(key) == 0 || pattern[0] != key[0] {
				return false
			}
			pattern, key = pattern[1:], key[1:]
		}
	}
	return len(key) == 0
}

// matchByteSet matches one byte against the body of a "[...]" set, starting
// just after the '['. It returns the pattern that follows the closing ']'. An
// unterminated set runs to the end of the pattern, as in Redis.
func matchByteSet(set string, candidate byte) (matched bool, rest string) {
	negated := len(set) > 0 && set[0] == '^'
	if negated {
		set = set[1:]
	}
	for len(set) > 0 && set[0] != ']' {
		switch {
		case set[0] == '\\' && len(set) > 1:
			matched = matched || set[1] == candidate
			set = set[2:]
		case len(set) > 2 && set[1] == '-':
			low, high := set[0], set[2]
			if low > high {
				low, high = high, low
			}
			matched = matched || (candidate >= low && candidate <= high)
			set = set[3:]
		default:
			matched = matched || set[0] == candidate
			set = set[1:]
		}
	}
	if len(set) > 0 {
		set = set[1:]
	}
	return matched != negated, set
}
