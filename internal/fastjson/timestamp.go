package fastjson

// Timestamp captures the components of an ISO-8601 UTC timestamp the
// vectorizer needs without going through time.Time. The contract sends
// timestamps in the form YYYY-MM-DDTHH:MM:SSZ, optionally with a
// fractional-second component (e.g. 2026-03-11T03:45:53.123Z) — the
// fraction is parsed but discarded since it lives below the resolution
// we feed into the vector (minute/hour/weekday).
type Timestamp struct {
	UnixSeconds int64
	Hour        int
	WeekdayMon0 int // Monday=0 ... Sunday=6
}

// parseTimestamp parses a fixed-form UTC timestamp from s. Any deviation
// from YYYY-MM-DDTHH:MM:SS(.fff)?Z returns false so the caller can fall
// back to encoding/json + time.Parse.
func parseTimestamp(s []byte) (Timestamp, bool) {
	// Minimum length is "2006-01-02T15:04:05Z" = 20 chars.
	if len(s) < 20 {
		return Timestamp{}, false
	}
	if s[4] != '-' || s[7] != '-' || s[10] != 'T' ||
		s[13] != ':' || s[16] != ':' {
		return Timestamp{}, false
	}
	year, ok := readDigits(s, 0, 4)
	if !ok {
		return Timestamp{}, false
	}
	month, ok := readDigits(s, 5, 2)
	if !ok || month < 1 || month > 12 {
		return Timestamp{}, false
	}
	day, ok := readDigits(s, 8, 2)
	if !ok || day < 1 || day > 31 {
		return Timestamp{}, false
	}
	hour, ok := readDigits(s, 11, 2)
	if !ok || hour > 23 {
		return Timestamp{}, false
	}
	minute, ok := readDigits(s, 14, 2)
	if !ok || minute > 59 {
		return Timestamp{}, false
	}
	second, ok := readDigits(s, 17, 2)
	if !ok || second > 59 {
		return Timestamp{}, false
	}
	i := 19
	if i < len(s) && s[i] == '.' {
		i++
		j := i
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
		if i == j {
			return Timestamp{}, false
		}
	}
	if i >= len(s) || s[i] != 'Z' {
		return Timestamp{}, false
	}
	if i+1 != len(s) {
		return Timestamp{}, false
	}

	days := daysFromCivil(year, month, day)
	unix := int64(days)*86400 + int64(hour)*3600 + int64(minute)*60 + int64(second)
	// 1970-01-01 was a Thursday; Go's time.Weekday counts Sunday=0,
	// so Thursday=4. weekdaySun0 = (days+4) mod 7.
	weekdaySun0 := ((days + 4) % 7 + 7) % 7
	weekdayMon0 := (weekdaySun0 + 6) % 7
	return Timestamp{
		UnixSeconds: unix,
		Hour:        hour,
		WeekdayMon0: weekdayMon0,
	}, true
}

func readDigits(s []byte, off, n int) (int, bool) {
	if off+n > len(s) {
		return 0, false
	}
	v := 0
	for i := 0; i < n; i++ {
		c := s[off+i]
		if c < '0' || c > '9' {
			return 0, false
		}
		v = v*10 + int(c-'0')
	}
	return v, true
}

// daysFromCivil returns the number of days from the proleptic Gregorian
// 1970-01-01. Implementation lifted from Howard Hinnant's date library
// (public domain), trimmed for positive years which is all the contract
// produces.
func daysFromCivil(y, m, d int) int {
	if m <= 2 {
		y--
	}
	era := y / 400
	if y < 0 && y%400 != 0 {
		era--
	}
	yoe := y - era*400
	var moe int
	if m > 2 {
		moe = m - 3
	} else {
		moe = m + 9
	}
	doy := (153*moe+2)/5 + d - 1
	doe := yoe*365 + yoe/4 - yoe/100 + doy
	return era*146097 + doe - 719468
}
