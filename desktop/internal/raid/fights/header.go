package fights

import "strconv"

// Header.Raw is a version followed by fixed key/value pairs, not a combat line.
func readHeader(r *Run, raw []string) {
	if len(raw) == 0 {
		return
	}
	r.LogVersion, _ = strconv.ParseInt(raw[0], 10, 64)
	for i := 1; i+1 < len(raw); i += 2 {
		switch raw[i] {
		case "BUILD_VERSION":
			r.Build = raw[i+1]
		case "ADVANCED_LOG_ENABLED":
			r.Advanced = raw[i+1] == "1"
		}
	}
}
