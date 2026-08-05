package feel

// refer to https://kiegroup.github.io/dmn-feel-handbook/#date
// refer to https://docs.camunda.io/docs/components/modeler/feel/language-guide/feel-temporal-expressions/

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var ErrParseTemporal = errors.New("fail to parse temporal value")

type HasTime interface {
	Time() time.Time
}

type HasDate interface {
	Date() time.Time
}

// zoneKind distinguishes how a time/date-time literal expressed its zone:
// not at all, as a numeric offset (including "Z"), or as a named IANA zone
// (e.g. "@Australia/Melbourne"). Two temporal values that represent the same
// instant are only `is`-equal (and print identically) if their zone is
// expressed the same way.
const (
	zoneNone = iota
	zoneOffset
	zoneNamed
)

var offsetSuffixPattern = regexp.MustCompile(`([+-])(\d{2}):(\d{2})(?::\d{2})?$`)

// isZoned reports whether a temporal value carries explicit zone/offset
// info. A date has none of its own but implies UTC, so it is always
// treated as zoned; a date-time is zoned only if it was parsed with an
// offset or named zone.
func isZoned(v any) bool {
	switch vv := v.(type) {
	case *FEELDate:
		return true
	case *FEELDatetime:
		return vv.zoneKind != zoneNone
	default:
		return false
	}
}

// classifyZone inspects a raw temporal literal's text and reports how (if at
// all) it expressed a time zone.
func classifyZone(src string) (kind int, name string) {
	if idx := strings.Index(src, "@"); idx >= 0 {
		return zoneNamed, src[idx+1:]
	}
	if strings.HasSuffix(src, "Z") {
		return zoneOffset, ""
	}
	if offsetSuffixPattern.MatchString(src) {
		return zoneOffset, ""
	}
	return zoneNone, ""
}

// formatZoned renders t (already parsed with its clock fields intact) using
// layout for the non-zone portion, then appends the appropriate zone suffix
// for kind/name.
func formatZoned(t time.Time, layout string, kind int, name string) string {
	s := t.Format(layout)
	switch kind {
	case zoneNamed:
		return s + "@" + name
	case zoneOffset:
		_, offsetSecs := t.Zone()
		switch {
		case offsetSecs == 0:
			return s + "Z"
		case offsetSecs%60 != 0:
			return s + t.Format("-07:00:00")
		default:
			return s + t.Format("-07:00")
		}
	default:
		return s
	}
}

// time
type FEELTime struct {
	t        time.Time
	zoneKind int
	zoneName string
}

func (st FEELTime) Time() time.Time {
	return st.t
}

var timePatterns = []string{
	"15:04:05.999999999Z07:00:00",
	"15:04:05.999999999Z07:00",
	"15:04:05.999999999-07:00:00",
	"15:04:05.999999999-07:00",
	"15:04:05.999999999",
	"15:04:05Z07:00:00",
	"15:04:05Z07:00",
	"15:04:05-07:00:00",
	"15:04:05-07:00",
	"15:04:05@MST",
	"15:04:05",
}

// clockPrefixPattern requires exactly two digits each for hour, minute, and
// second (FEEL/XSD time literals reject "7:00:00", unlike Go's time.Parse).
var clockPrefixPattern = regexp.MustCompile(`^\d{2}:\d{2}:\d{2}`)

const maxZoneOffsetSeconds = 14 * 3600

func validOffsetSeconds(t time.Time) bool {
	_, offset := t.Zone()
	return offset >= -maxZoneOffsetSeconds && offset <= maxZoneOffsetSeconds
}

func ParseTime(temporalStr string) (*FEELTime, error) {
	if !clockPrefixPattern.MatchString(temporalStr) {
		return nil, ErrParseTemporal
	}
	if atIdx := strings.Index(temporalStr, "@"); atIdx > 0 {
		timePart := temporalStr[:atIdx]
		tzName := temporalStr[atIdx+1:]
		if strings.Contains(tzName, "/") || len(tzName) > 3 {
			if loc, err := time.LoadLocation(tzName); err == nil {
				for _, pat := range []string{"15:04:05.999999999", "15:04:05"} {
					if t, err := time.ParseInLocation(pat, timePart, loc); err == nil {
						return &FEELTime{t: t, zoneKind: zoneNamed, zoneName: tzName}, nil
					}
				}
			}
			return nil, ErrParseTemporal
		}
	}
	for _, pat := range timePatterns {
		if t, err := time.Parse(pat, temporalStr); err == nil {
			if !validOffsetSeconds(t) {
				return nil, ErrParseTemporal
			}
			kind, name := classifyZone(temporalStr)
			return &FEELTime{t: t, zoneKind: kind, zoneName: name}, nil
		}
	}
	return nil, ErrParseTemporal
}

func (st FEELTime) GetAttr(name string) (any, bool) {
	switch name {
	case "hour":
		return N(st.t.Hour()), true
	case "minute":
		return N(st.t.Minute()), true
	case "second":
		return N(st.t.Second()), true
	case "timezone":
		if st.zoneKind != zoneNamed {
			return Null, true
		}
		return st.zoneName, true
	case "time offset":
		if st.zoneKind == zoneNone {
			return Null, true
		}
		_, offsetSecs := st.t.Zone()
		return NewFEELDuration(time.Duration(offsetSecs) * time.Second), true
	}
	return nil, false
}

func (st FEELTime) MarshalJSON() ([]byte, error) {
	return json.Marshal(st.String())
}

func (st FEELTime) String() string {
	layout := "15:04:05"
	if st.t.Nanosecond() != 0 {
		layout = "15:04:05.999999999"
	}
	return formatZoned(st.t, layout, st.zoneKind, st.zoneName)
}

func (st *FEELTime) Add(dur *FEELDuration) *FEELTime {
	return &FEELTime{t: st.t.Add(dur.Duration()), zoneKind: st.zoneKind, zoneName: st.zoneName}
}

func (st *FEELTime) Sub(other HasTime) *FEELDuration {
	return NewFEELDuration(st.t.Sub(other.Time()))
}

// Date
type FEELDate struct {
	t time.Time
}

func (date FEELDate) Date() time.Time {
	return date.t
}

func (date FEELDate) Time() time.Time {
	return date.t
}

func (date FEELDate) GetAttr(name string) (any, bool) {
	switch name {
	case "year":
		return N(date.t.Year()), true
	case "month":
		return N(int(date.t.Month())), true
	case "day":
		return N(date.t.Day()), true
	case "weekday":
		return N(isoWeekday(date.t)), true
	}
	return nil, false
}

// isoWeekday converts Go's Sunday=0..Saturday=6 weekday into FEEL's ISO 8601
// Monday=1..Sunday=7 convention.
func isoWeekday(t time.Time) int {
	wd := int(t.Weekday())
	if wd == 0 {
		wd = 7
	}
	return wd
}

func (date FEELDate) String() string {
	return date.t.Format("2006-01-02")
}

func (date FEELDate) MarshalJSON() ([]byte, error) {
	return json.Marshal(date.String())
}

func (date *FEELDate) Add(dur *FEELDuration) *FEELDate {
	if dur.IsYM {
		durMonths := dur.TotalMonths()
		totalMonths := int64(date.t.Year())*12 + int64(date.t.Month()-1) + durMonths
		year := totalMonths / 12
		month := totalMonths % 12
		if month < 0 {
			month += 12
			year--
		}
		return &FEELDate{t: time.Date(int(year), time.Month(month+1), date.t.Day(), 0, 0, 0, 0, date.t.Location())}
	}
	return &FEELDate{t: date.t.Add(dur.Duration())}
}

func (date *FEELDate) Sub(other HasDate) *FEELDuration {
	return NewFEELDuration(date.t.Sub(other.Date()))
}

var datePatterns = []string{
	"2006-01-02",
}

func ParseDate(timeStr string) (*FEELDate, error) {
	if len(timeStr) > 1 && timeStr[0] == '-' && timeStr[1] >= '0' && timeStr[1] <= '9' {
		d, err := ParseDate(timeStr[1:])
		if err != nil {
			return nil, err
		}
		negT := time.Date(-d.t.Year(), d.t.Month(), d.t.Day(), 0, 0, 0, 0, d.t.Location())
		return &FEELDate{t: negT}, nil
	}
	for _, pat := range datePatterns {
		if t, err := time.Parse(pat, timeStr); err == nil {
			return &FEELDate{t: t}, nil
		}
	}
	if m := largeYearDateRe.FindStringSubmatch(timeStr); m != nil {
		if len(m[1]) > 4 && m[1][0] == '0' {
			return nil, ErrParseTemporal
		}
		year, _ := strconv.Atoi(m[1])
		month, _ := strconv.Atoi(m[2])
		day, _ := strconv.Atoi(m[3])
		if month < 1 || month > 12 || day < 1 || day > 31 {
			return nil, ErrParseTemporal
		}
		t := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
		if t.Year() != year || int(t.Month()) != month || t.Day() != day {
			return nil, ErrParseTemporal
		}
		return &FEELDate{t: t}, nil
	}
	return nil, ErrParseTemporal
}

// largeYearDateRe matches date strings with 4-9 digit years (Go's "2006-01-02" layout only accepts exactly 4).
var largeYearDateRe = regexp.MustCompile(`^(\d{4,9})-(\d{2})-(\d{2})$`)

// Date and Time
type FEELDatetime struct {
	t        time.Time
	zoneKind int
	zoneName string
}

func (sdt FEELDatetime) Time() time.Time {
	return sdt.t
}

func (sdt FEELDatetime) Date() time.Time {
	return sdt.t
}

func (sdt FEELDatetime) Equal(other FEELDatetime) bool {
	return sdt.t.Equal(other.t)
}

func (sdt FEELDatetime) Compare(other FEELDatetime) int {
	if sdt.t.Equal(other.t) {
		return 0
	} else if sdt.t.Before(other.t) {
		return -1
	} else {
		return 1
	}
}

func (sdt FEELDatetime) GetAttr(name string) (any, bool) {
	switch name {
	case "year":
		return N(sdt.t.Year()), true
	case "month":
		return N(int(sdt.t.Month())), true
	case "day":
		return N(sdt.t.Day()), true
	case "weekday":
		return N(isoWeekday(sdt.t)), true
	case "hour":
		return N(sdt.t.Hour()), true
	case "minute":
		return N(sdt.t.Minute()), true
	case "second":
		return N(sdt.t.Second()), true
	case "timezone":
		if sdt.zoneKind != zoneNamed {
			return Null, true
		}
		return sdt.zoneName, true
	case "time offset":
		if sdt.zoneKind == zoneNone {
			return Null, true
		}
		_, offsetSecs := sdt.t.Zone()
		return NewFEELDuration(time.Duration(offsetSecs) * time.Second), true
	}
	return nil, false
}

func (sdt FEELDatetime) MarshalJSON() ([]byte, error) {
	return json.Marshal(sdt.String())
}

func (sdt FEELDatetime) String() string {
	layout := "2006-01-02T15:04:05"
	if sdt.t.Nanosecond() != 0 {
		layout = "2006-01-02T15:04:05.999999999"
	}
	return formatZoned(sdt.t, layout, sdt.zoneKind, sdt.zoneName)
}

func (sdt *FEELDatetime) Add(dur *FEELDuration) *FEELDatetime {
	if dur.Years > 0 || dur.Months > 0 {
		durMonths := dur.Years*12 + dur.Months
		timeMonths := sdt.t.Year()*12 + int(sdt.t.Month()-1)

		newTimeMonths := timeMonths + durMonths
		if dur.Neg {
			newTimeMonths = timeMonths - durMonths
		}
		return &FEELDatetime{
			t: time.Date(
				newTimeMonths/12, time.Month(newTimeMonths%12+1),
				sdt.t.Day(), sdt.t.Hour(), sdt.t.Minute(),
				sdt.t.Second(), sdt.t.Nanosecond(),
				sdt.t.Location()),
			zoneKind: sdt.zoneKind,
			zoneName: sdt.zoneName,
		}
	}
	return &FEELDatetime{t: sdt.t.Add(dur.Duration()), zoneKind: sdt.zoneKind, zoneName: sdt.zoneName}
}

func (sdt *FEELDatetime) Sub(v HasTime) *FEELDuration {
	delta := sdt.t.Sub(v.Time())
	return NewFEELDuration(delta)
}

var dateTimePatterns = []string{
	"2006-01-02T15:04:05.999999999Z07:00:00",
	"2006-01-02T15:04:05.999999999Z07:00",
	"2006-01-02T15:04:05.999999999-07:00:00",
	"2006-01-02T15:04:05.999999999-07:00",
	"2006-01-02T15:04:05.999999999",
	"2006-01-02T15:04:05Z07:00:00",
	"2006-01-02T15:04:05Z07:00",
	"2006-01-02T15:04:05-07:00:00",
	"2006-01-02T15:04:05-07:00",
	"2006-01-02T15:04:05@MST",
	"2006-01-02T15:04:05",
}

// largeYearDTRe matches datetime strings with 4-9 digit years, with an
// optional trailing "Z"/numeric-offset suffix (no named "@zone" - that's
// handled separately via the IANA path).
var largeYearDTRe = regexp.MustCompile(`^(\d{4,9})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2})(?:\.(\d+))?(Z|[+-]\d{2}:\d{2}(?::\d{2})?)?$`)

// parseLargeYearOffsetDatetime parses a large-year datetime string,
// deriving its location from a trailing "Z"/numeric-offset suffix if
// present (defaulting to UTC otherwise).
func parseLargeYearOffsetDatetime(s string) (time.Time, bool) {
	m := largeYearDTRe.FindStringSubmatch(s)
	if m == nil {
		return time.Time{}, false
	}
	loc := time.UTC
	if offsetStr := m[8]; offsetStr != "" && offsetStr != "Z" {
		sign := 1
		if offsetStr[0] == '-' {
			sign = -1
		}
		parts := strings.Split(offsetStr[1:], ":")
		hh, _ := strconv.Atoi(parts[0])
		mm, _ := strconv.Atoi(parts[1])
		ss := 0
		if len(parts) > 2 {
			ss, _ = strconv.Atoi(parts[2])
		}
		loc = time.FixedZone("", sign*(hh*3600+mm*60+ss))
	}
	return parseLargeYearDatetime(s, loc)
}

func parseLargeYearDatetime(s string, loc *time.Location) (time.Time, bool) {
	m := largeYearDTRe.FindStringSubmatch(s)
	if m == nil {
		return time.Time{}, false
	}
	if len(m[1]) > 4 && m[1][0] == '0' {
		return time.Time{}, false
	}
	year, _ := strconv.Atoi(m[1])
	month, _ := strconv.Atoi(m[2])
	day, _ := strconv.Atoi(m[3])
	hour, _ := strconv.Atoi(m[4])
	min, _ := strconv.Atoi(m[5])
	sec, _ := strconv.Atoi(m[6])
	if month < 1 || month > 12 || day < 1 || day > 31 || hour > 23 || min > 59 || sec > 59 {
		return time.Time{}, false
	}
	nsec := 0
	if m[7] != "" {
		frac := m[7]
		if len(frac) > 9 {
			frac = frac[:9]
		}
		for len(frac) < 9 {
			frac += "0"
		}
		nsec, _ = strconv.Atoi(frac)
	}
	t := time.Date(year, time.Month(month), day, hour, min, sec, nsec, loc)
	if t.Year() != year || int(t.Month()) != month || t.Day() != day {
		return time.Time{}, false
	}
	return t, true
}

// midnight24Pattern matches the ISO 8601 "T24:00:00" representation of
// midnight at the end of a day (only valid with zero minutes/seconds).
var midnight24Pattern = regexp.MustCompile(`^(\d{4,9}-\d{2}-\d{2})T24:00:00(\.0+)?(.*)$`)

// normalizeMidnight24 rewrites a "T24:00:00" datetime (Go's time.Parse
// rejects hour 24) into the equivalent "T00:00:00" on the following day,
// preserving any trailing offset/zone suffix.
func normalizeMidnight24(temporalStr string) string {
	m := midnight24Pattern.FindStringSubmatch(temporalStr)
	if m == nil {
		return temporalStr
	}
	t, err := time.Parse("2006-01-02", m[1])
	if err != nil {
		return temporalStr
	}
	next := t.AddDate(0, 0, 1)
	return next.Format("2006-01-02") + "T00:00:00" + m[3]
}

func ParseDatetime(temporalStr string) (*FEELDatetime, error) {
	// Handle negative years like "-2016-01-30T09:05:00"
	if len(temporalStr) > 1 && temporalStr[0] == '-' && temporalStr[1] >= '0' && temporalStr[1] <= '9' {
		dt, err := ParseDatetime(temporalStr[1:])
		if err != nil {
			return nil, err
		}
		t := dt.t
		negT := time.Date(-t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), t.Location())
		return &FEELDatetime{t: negT, zoneKind: dt.zoneKind, zoneName: dt.zoneName}, nil
	}
	// date-only: normalize to YYYY-MM-DDTHH:MM:SS
	if matched, _ := regexp.MatchString(`^\d{4}-\d{2}-\d{2}$`, temporalStr); matched {
		if t, err := time.Parse("2006-01-02", temporalStr); err == nil {
			return &FEELDatetime{t: t}, nil
		}
	}
	temporalStr = normalizeMidnight24(temporalStr)
	if !datetimeClockPattern.MatchString(temporalStr) {
		return nil, ErrParseTemporal
	}
	// @IANA timezone with slash or long name (e.g. Etc/UTC)
	if atIdx := strings.LastIndex(temporalStr, "@"); atIdx > 10 {
		tzName := temporalStr[atIdx+1:]
		if strings.Contains(tzName, "/") || len(tzName) > 3 {
			dtPart := temporalStr[:atIdx]
			if loc, err := time.LoadLocation(tzName); err == nil {
				for _, pat := range []string{"2006-01-02T15:04:05.999999999", "2006-01-02T15:04:05"} {
					if t, err := time.ParseInLocation(pat, dtPart, loc); err == nil {
						return &FEELDatetime{t: t, zoneKind: zoneNamed, zoneName: tzName}, nil
					}
				}
				// A numeric offset combined with a named zone (e.g.
				// "...+01:00@Europe/Paris") is invalid.
				if m := largeYearDTRe.FindStringSubmatch(dtPart); m == nil || m[8] == "" {
					if t, ok := parseLargeYearDatetime(dtPart, loc); ok {
						return &FEELDatetime{t: t, zoneKind: zoneNamed, zoneName: tzName}, nil
					}
				}
			}
			return nil, ErrParseTemporal
		}
	}
	for _, pat := range dateTimePatterns {
		if t, err := time.Parse(pat, temporalStr); err == nil {
			if !validOffsetSeconds(t) {
				return nil, ErrParseTemporal
			}
			kind, name := classifyZone(temporalStr)
			return &FEELDatetime{t: t, zoneKind: kind, zoneName: name}, nil
		}
	}
	// years outside the 4-digit range Go's time.Parse layouts require
	if t, ok := parseLargeYearOffsetDatetime(temporalStr); ok {
		if !validOffsetSeconds(t) {
			return nil, ErrParseTemporal
		}
		kind, name := classifyZone(temporalStr)
		return &FEELDatetime{t: t, zoneKind: kind, zoneName: name}, nil
	}
	return nil, ErrParseTemporal
}

// datetimeClockPattern requires exactly two digits each for hour, minute,
// and second following the "T" (FEEL/XSD datetime literals reject
// "...T7:00:00", unlike Go's time.Parse).
var datetimeClockPattern = regexp.MustCompile(`T\d{2}:\d{2}:\d{2}`)

func MustParseDatetime(temporalStr string) *FEELDatetime {
	t, err := ParseDatetime(temporalStr)
	if err != nil {
		panic(err)
	}
	return t
}

type FEELDuration struct {
	IsYM        bool
	Neg         bool
	Years       int
	Months      int
	Days        int
	Hours       int
	Minutes     int
	Seconds     int
	SecondsFrac string // e.g. ".1234" when present
}

func NewFEELDuration(dur time.Duration) *FEELDuration {
	d := &FEELDuration{}
	ndur := int(dur)
	if ndur < 0 {
		d.Neg = true
		ndur = -ndur
	}
	nhours := ndur / int(time.Hour)
	remain := ndur - nhours*int(time.Hour)
	nmins := remain / int(time.Minute)

	remain -= nmins * int(time.Minute)
	nsecs := remain / int(time.Second)

	d.Days = nhours / 24
	d.Hours = nhours - d.Days*24
	d.Minutes = nmins
	d.Seconds = nsecs
	return d
}

func (dur FEELDuration) GetAttr(name string) (any, bool) {
	// years/months only exist on a year-month duration; days/hours/minutes/
	// seconds only exist on a day-time duration - the other kind's fields
	// aren't just zero, they're not a property of that value at all.
	if dur.IsYM {
		switch name {
		case "years":
			return dur.Years, true
		case "months":
			return dur.Months, true
		}
		return nil, false
	}
	switch name {
	case "days":
		return dur.Days, true
	case "hours":
		return dur.Hours, true
	case "minutes":
		return dur.Minutes, true
	case "seconds":
		return dur.Seconds, true
	}
	return nil, false
}

func (dur FEELDuration) MarshalJSON() ([]byte, error) {
	return json.Marshal(dur.String())
}

func (dur FEELDuration) Duration() time.Duration {
	// dur.Year and dur.Month
	dv := (time.Duration(dur.Days*24+dur.Hours)*time.Hour +
		time.Duration(dur.Minutes)*time.Minute +
		time.Duration(dur.Seconds)*time.Second)
	if dur.Neg {
		dv = -dv
	}
	return dv
}

func (dur FEELDuration) IsYearMonth() bool {
	return dur.IsYM
}

func (dur FEELDuration) TotalMonths() int64 {
	total := int64(dur.Years)*12 + int64(dur.Months)
	if dur.Neg {
		return -total
	}
	return total
}

func (dur *FEELDuration) Negative() *FEELDuration {
	neg := *dur
	neg.Neg = !dur.Neg
	return &neg
}

func (dur *FEELDuration) Add(other *FEELDuration) (*FEELDuration, error) {
	if dur.IsYM != other.IsYM {
		return nil, errors.New("cannot add year-month and day-time durations")
	}
	if dur.IsYM {
		return NewYearMonthDuration(dur.TotalMonths() + other.TotalMonths()), nil
	}
	return NewFEELDuration(dur.Duration() + other.Duration()), nil
}

func (dur *FEELDuration) Sub(other *FEELDuration) (*FEELDuration, error) {
	return dur.Add(other.Negative())
}

func (dur *FEELDuration) MulNumber(n *Number) (*FEELDuration, error) {
	if dur.IsYM {
		return NewYearMonthDuration(int64(float64(dur.TotalMonths()) * n.Float64())), nil
	}
	return NewFEELDuration(time.Duration(float64(dur.Duration()) * n.Float64())), nil
}

func (dur *FEELDuration) DivNumber(n *Number) (*FEELDuration, error) {
	if n.IsZero() {
		return nil, errors.New("division by zero")
	}
	if dur.IsYM {
		return NewYearMonthDuration(int64(float64(dur.TotalMonths()) / n.Float64())), nil
	}
	return NewFEELDuration(time.Duration(float64(dur.Duration()) / n.Float64())), nil
}

func (dur *FEELDuration) DivDuration(other *FEELDuration) (*Number, error) {
	if dur.IsYM != other.IsYM {
		return nil, errors.New("cannot divide year-month and day-time durations")
	}
	if dur.IsYM {
		if other.TotalMonths() == 0 {
			return nil, errors.New("division by zero")
		}
		return NewNumberFromFloat(float64(dur.TotalMonths()) / float64(other.TotalMonths())), nil
	}
	if other.Duration() == 0 {
		return nil, errors.New("division by zero")
	}
	return NewNumberFromFloat(float64(dur.Duration()) / float64(other.Duration())), nil
}

// NewYearMonthDuration builds a year-month duration from a total month count.
func NewYearMonthDuration(totalMonths int64) *FEELDuration {
	dur := &FEELDuration{IsYM: true}
	if totalMonths < 0 {
		dur.Neg = true
		totalMonths = -totalMonths
	}
	dur.Years = int(totalMonths / 12)
	dur.Months = int(totalMonths % 12)
	return dur
}

// secondsFracIsZero reports whether a parsed fractional-seconds suffix (e.g.
// "", ".000", ".") represents no fractional value at all.
func secondsFracIsZero(frac string) bool {
	for _, c := range frac {
		if c != '.' && c != '0' {
			return false
		}
	}
	return true
}

func (dur FEELDuration) String() string {
	neg := ""
	if dur.Neg {
		neg = "-"
	}
	if dur.IsYM {
		if dur.Years == 0 && dur.Months == 0 {
			return neg + "P0M"
		}
		s := ""
		if dur.Years != 0 {
			s += fmt.Sprintf("%dY", dur.Years)
		}
		if dur.Months != 0 {
			s += fmt.Sprintf("%dM", dur.Months)
		}
		return neg + "P" + s
	}
	// day-time duration
	if dur.Days == 0 && dur.Hours == 0 && dur.Minutes == 0 && dur.Seconds == 0 && secondsFracIsZero(dur.SecondsFrac) {
		return neg + "PT0S"
	}
	sDay, sTime := "", ""
	if dur.Days != 0 {
		sDay = fmt.Sprintf("%dD", dur.Days)
	}
	if dur.Hours != 0 {
		sTime += fmt.Sprintf("%dH", dur.Hours)
	}
	if dur.Minutes != 0 {
		sTime += fmt.Sprintf("%dM", dur.Minutes)
	}
	frac := dur.SecondsFrac
	if secondsFracIsZero(frac) {
		frac = ""
	}
	if dur.Seconds != 0 || frac != "" {
		sTime += fmt.Sprintf("%d%sS", dur.Seconds, frac)
	}
	if sTime != "" {
		return neg + "P" + sDay + "T" + sTime
	}
	return neg + "P" + sDay
}

var yearmonthDurationPattern = regexp.MustCompile(`^(\-?)P((\d+)Y)?((\d+)M)?$`)

// groups: [1]=neg [2]=dayspart [3]=daysval [4]=timepart [5]=hourspart [6]=hoursval
//
//	[7]=minspart [8]=minsval [9]=secspart [10]=secsval [11]=secsfrac
var timeDurationPattern = regexp.MustCompile(`^(\-?)P((\d+)D)?(T((\d+)H)?((\d+)M)?((\d+)(\.\d*)?S)?)?$`)

func ParseDuration(temporalStr string) (*FEELDuration, error) {
	// parse year-month duration
	if submatches := yearmonthDurationPattern.FindStringSubmatch(temporalStr); submatches != nil {
		if submatches[2] == "" && submatches[4] == "" {
			return nil, ErrParseTemporal
		}
		dur := &FEELDuration{IsYM: true}
		if submatches[1] != "" {
			dur.Neg = true
		}
		if submatches[2] != "" {
			y, err := strconv.ParseInt(submatches[3], 10, 64)
			if err != nil {
				return nil, err
			}
			dur.Years = int(y)
		}
		if submatches[4] != "" {
			m, err := strconv.ParseInt(submatches[5], 10, 64)
			if err != nil {
				return nil, err
			}
			dur.Months = int(m)
			// normalize months >= 12 into years
			dur.Years += dur.Months / 12
			dur.Months = dur.Months % 12
		}
		return dur, nil
	}

	// parse day-time duration
	if submatches := timeDurationPattern.FindStringSubmatch(temporalStr); submatches != nil {
		if submatches[2] == "" && submatches[5] == "" && submatches[7] == "" && submatches[9] == "" {
			return nil, ErrParseTemporal
		}
		dur := &FEELDuration{}
		if submatches[1] != "" {
			dur.Neg = true
		}
		if submatches[2] != "" {
			v, err := strconv.ParseInt(submatches[3], 10, 64)
			if err != nil {
				return nil, err
			}
			dur.Days = int(v)
		}
		if submatches[5] != "" {
			v, err := strconv.ParseInt(submatches[6], 10, 64)
			if err != nil {
				return nil, err
			}
			dur.Hours = int(v)
		}
		if submatches[7] != "" {
			v, err := strconv.ParseInt(submatches[8], 10, 64)
			if err != nil {
				return nil, err
			}
			dur.Minutes = int(v)
		}
		if submatches[9] != "" {
			v, err := strconv.ParseInt(submatches[10], 10, 64)
			if err != nil {
				return nil, err
			}
			dur.Seconds = int(v)
			frac := submatches[11]
			if frac == "." {
				frac = ""
			}
			dur.SecondsFrac = frac // ".1234" or ""
		}

		// Carry seconds -> minutes -> hours -> days, since each field is
		// parsed independently and the source text isn't required to keep
		// them within their usual bounds (e.g. "PT61M" or "PT3600S").
		dur.Minutes += dur.Seconds / 60
		dur.Seconds = dur.Seconds % 60
		dur.Hours += dur.Minutes / 60
		dur.Minutes = dur.Minutes % 60
		dur.Days += dur.Hours / 24
		dur.Hours = dur.Hours % 24

		return dur, nil
	}

	return nil, ErrParseTemporal
}

func MustParseDuration(s string) *FEELDuration {
	d, err := ParseDuration(s)
	if err != nil {
		panic(err)
	}
	return d
}

func ParseTemporalValue(temporalStr string) (any, error) {
	// ParseDate is tried first and only matches a strict date-only literal
	// (YYYY-MM-DD); ParseDatetime would otherwise also accept it (defaulting
	// the time to midnight), which is wrong for a `@"..."` literal.
	if v, err := ParseDate(temporalStr); err == nil {
		return v, nil
	}

	if v, err := ParseDatetime(temporalStr); err == nil {
		return v, nil
	}

	if v, err := ParseTime(temporalStr); err == nil {
		return v, nil
	}

	return ParseDuration(temporalStr)
}

// builtin functions
// dateFromValue implements the single-argument form of date(from): a
// string is parsed as a date literal; a date/date-time value has its date
// component extracted (dropping any time-of-day).
func dateFromValue(eval func(Node) (any, error), fromNode Node) (any, error) {
	fromVal, err := eval(fromNode)
	if err != nil {
		return nil, err
	}
	if _, isNull := fromVal.(*NullValue); isNull {
		return Null, nil
	}
	if s, ok := fromVal.(string); ok {
		d, err := ParseDate(s)
		if err != nil {
			return Null, nil
		}
		return d, nil
	}
	if hasDate, ok := fromVal.(HasDate); ok {
		d := hasDate.Date()
		return &FEELDate{t: time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)}, nil
	}
	return Null, nil
}

func installDatetimeFunctions(prelude *Prelude) {
	// conversions
	prelude.Bind("date", NewRawFunc(func(intp *Interpreter, node FunCall) (any, error) {
		eval := func(n Node) (any, error) { return n.Eval(intp) }

		// date(year, month, day) / date(year: ..., month: ..., day: ...)
		byParts := func(yearNode, monthNode, dayNode Node) (any, error) {
			yearVal, err := eval(yearNode)
			if err != nil {
				return nil, err
			}
			monthVal, err := eval(monthNode)
			if err != nil {
				return nil, err
			}
			dayVal, err := eval(dayNode)
			if err != nil {
				return nil, err
			}
			year, ok := yearVal.(*Number)
			if !ok {
				return Null, nil
			}
			month, ok := monthVal.(*Number)
			if !ok {
				return Null, nil
			}
			day, ok := dayVal.(*Number)
			if !ok {
				return Null, nil
			}
			y, m, d := year.Int(), month.Int(), day.Int()
			if y < -999999999 || y > 999999999 || m < 1 || m > 12 || d < 1 || d > 31 {
				return Null, nil
			}
			t := time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)
			if t.Year() != y || int(t.Month()) != m || t.Day() != d {
				return Null, nil
			}
			return &FEELDate{t: t}, nil
		}

		if node.keywordArgs {
			kwArgMap := make(map[string]Node)
			for _, a := range node.Args {
				kwArgMap[a.argName] = a.arg
			}
			if len(kwArgMap) == 1 {
				if fromNode, ok := kwArgMap["from"]; ok {
					return dateFromValue(eval, fromNode)
				}
				return Null, nil
			}
			if len(kwArgMap) == 3 {
				yearNode, ok1 := kwArgMap["year"]
				monthNode, ok2 := kwArgMap["month"]
				dayNode, ok3 := kwArgMap["day"]
				if ok1 && ok2 && ok3 {
					return byParts(yearNode, monthNode, dayNode)
				}
			}
			return Null, nil
		}

		switch len(node.Args) {
		case 1:
			return dateFromValue(eval, node.Args[0].arg)
		case 3:
			return byParts(node.Args[0].arg, node.Args[1].arg, node.Args[2].arg)
		default:
			return Null, nil
		}
	}))

	prelude.Bind("time", NewRawFunc(func(intp *Interpreter, node FunCall) (any, error) {
		eval := func(n Node) (any, error) { return n.Eval(intp) }

		// time(from): a string is parsed as a time literal; a date/time/
		// date-time value has its time-of-day component extracted (a bare
		// date has no time of day, so it implies midnight UTC).
		fromValue := func(fromNode Node) (any, error) {
			v, err := eval(fromNode)
			if err != nil {
				return nil, err
			}
			if _, isNull := v.(*NullValue); isNull {
				return Null, nil
			}
			switch vv := v.(type) {
			case string:
				t, err := ParseTime(vv)
				if err != nil {
					return Null, nil
				}
				return t, nil
			case *FEELTime:
				return &FEELTime{t: vv.t, zoneKind: vv.zoneKind, zoneName: vv.zoneName}, nil
			case *FEELDatetime:
				return &FEELTime{t: vv.t, zoneKind: vv.zoneKind, zoneName: vv.zoneName}, nil
			case *FEELDate:
				return &FEELTime{t: time.Date(0, 1, 1, 0, 0, 0, 0, time.UTC), zoneKind: zoneOffset}, nil
			default:
				return Null, nil
			}
		}

		// time(hour, minute, second[, offset]): offset is a day-time
		// duration giving the zone offset from UTC, or null/omitted for no
		// zone info.
		byParts := func(hourNode, minuteNode, secondNode, offsetNode Node) (any, error) {
			hourVal, err := eval(hourNode)
			if err != nil {
				return nil, err
			}
			minuteVal, err := eval(minuteNode)
			if err != nil {
				return nil, err
			}
			secondVal, err := eval(secondNode)
			if err != nil {
				return nil, err
			}
			hourN, ok := hourVal.(*Number)
			if !ok {
				return Null, nil
			}
			minuteN, ok := minuteVal.(*Number)
			if !ok {
				return Null, nil
			}
			secondN, ok := secondVal.(*Number)
			if !ok {
				return Null, nil
			}
			hour, minute, second := hourN.Int(), minuteN.Int(), secondN.Float64()
			if hour < 0 || hour > 23 || minute < 0 || minute > 59 || second < 0 || second >= 60 {
				return Null, nil
			}
			secInt := int(second)
			nsec := int((second - float64(secInt)) * 1e9)

			loc := time.UTC
			zoneKind := zoneNone
			if offsetNode != nil {
				offsetVal, err := eval(offsetNode)
				if err != nil {
					return nil, err
				}
				if _, isNull := offsetVal.(*NullValue); !isNull {
					dur, ok := offsetVal.(*FEELDuration)
					if !ok || dur.IsYearMonth() {
						return Null, nil
					}
					loc = time.FixedZone("", int(dur.Duration().Seconds()))
					zoneKind = zoneOffset
				}
			}
			t := time.Date(0, 1, 1, hour, minute, secInt, nsec, loc)
			return &FEELTime{t: t, zoneKind: zoneKind}, nil
		}

		if node.keywordArgs {
			kwArgMap := make(map[string]Node)
			for _, a := range node.Args {
				kwArgMap[a.argName] = a.arg
			}
			if len(kwArgMap) == 1 {
				if fromNode, ok := kwArgMap["from"]; ok {
					return fromValue(fromNode)
				}
				return Null, nil
			}
			hourNode, ok1 := kwArgMap["hour"]
			minuteNode, ok2 := kwArgMap["minute"]
			secondNode, ok3 := kwArgMap["second"]
			if (len(kwArgMap) == 3 || len(kwArgMap) == 4) && ok1 && ok2 && ok3 {
				offsetNode := kwArgMap["offset"]
				return byParts(hourNode, minuteNode, secondNode, offsetNode)
			}
			return Null, nil
		}

		switch len(node.Args) {
		case 1:
			return fromValue(node.Args[0].arg)
		case 3:
			return byParts(node.Args[0].arg, node.Args[1].arg, node.Args[2].arg, nil)
		case 4:
			return byParts(node.Args[0].arg, node.Args[1].arg, node.Args[2].arg, node.Args[3].arg)
		default:
			return Null, nil
		}
	}))

	prelude.Bind("date and time", NewNativeFunc(func(args map[string]any) (any, error) {
		_, hasExtra := args["__extra"]
		if hasExtra {
			return Null, nil
		}
		fromVal, hasFrom := args["from"]
		timeVal, hasTime := args["time"]
		if !hasFrom {
			return Null, nil
		}
		if _, isNull := fromVal.(*NullValue); isNull {
			return Null, nil
		}
		if !hasTime {
			s, ok := fromVal.(string)
			if !ok {
				return Null, nil
			}
			dt, err := ParseDatetime(s)
			if err != nil {
				return Null, nil
			}
			return dt, nil
		}
		if _, isNull := timeVal.(*NullValue); isNull {
			return Null, nil
		}
		hasDate, ok1 := fromVal.(HasDate)
		feelTime, ok2 := timeVal.(*FEELTime)
		if !ok1 || !ok2 {
			return Null, nil
		}
		d := hasDate.Date()
		t := feelTime.t
		combined := time.Date(d.Year(), d.Month(), d.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), t.Location())
		return &FEELDatetime{t: combined, zoneKind: feelTime.zoneKind, zoneName: feelTime.zoneName}, nil
	}).Optional("from", "time").Alias("from", "date").Vararg("__extra"))

	prelude.Bind("duration", wrapTyped(func(frm string) (any, error) {
		return ParseDuration(frm)
	}).Required("from"))

	// temporal functions
	prelude.Bind("now", NewNativeFunc(func(args map[string]any) (any, error) {
		if _, hasExtra := args["__extra"]; hasExtra {
			return Null, nil
		}
		return &FEELDatetime{t: time.Now()}, nil
	}).Vararg("__extra"))

	prelude.Bind("today", NewNativeFunc(func(args map[string]any) (any, error) {
		if _, hasExtra := args["__extra"]; hasExtra {
			return Null, nil
		}
		return &FEELDate{t: time.Now()}, nil
	}).Vararg("__extra"))

	prelude.Bind("day of week", wrapTyped(func(v HasDate) (any, error) {
		return v.Date().Weekday().String(), nil
	}).Required("date"))

	prelude.Bind("day of year", wrapTyped(func(v HasDate) (any, error) {
		return NewNumberFromInt64(int64(v.Date().YearDay())), nil
	}).Required("date"))

	prelude.Bind("week of year", wrapTyped(func(v HasDate) (any, error) {
		_, week := v.Date().ISOWeek()
		return NewNumberFromInt64(int64(week)), nil
	}).Required("date"))

	prelude.Bind("month of year", wrapTyped(func(v HasDate) (any, error) {
		return v.Date().Month().String(), nil
	}).Required("date"))

	// refs https://docs.camunda.io/docs/components/modeler/feel/builtin-functions/feel-built-in-functions-temporal/#last-day-of-monthdate
	prelude.Bind("last day of month", wrapTyped(func(v HasDate) (any, error) {
		month := v.Date().Month()
		year := v.Date().Year()
		if month == 12 {
			year++
			month = 1
		} else {
			month++
		}
		nextFirstDay := time.Date(year, month, 1, 0, 0, 0, 0, v.Date().Location())
		lastDay := nextFirstDay.Add(-24 * time.Hour) // 1 day before
		return lastDay.Day(), nil
	}).Required("date"))

	prelude.Bind("years and months duration", NewNativeFunc(func(args map[string]any) (any, error) {
		if _, hasExtra := args["__extra"]; hasExtra {
			return Null, nil
		}
		fromVal, hasFrom := args["from"]
		toVal, hasTo := args["to"]
		if !hasFrom || !hasTo {
			return Null, nil
		}
		if _, isNull := fromVal.(*NullValue); isNull {
			return Null, nil
		}
		if _, isNull := toVal.(*NullValue); isNull {
			return Null, nil
		}
		fromDate, okFrom := fromVal.(HasDate)
		toDate, okTo := toVal.(HasDate)
		if !okFrom || !okTo {
			return Null, nil
		}
		from := fromDate.Date()
		to := toDate.Date()

		fromYear := from.Year()
		fromMonth := int(from.Month())
		fromDay := from.Day()
		toYear := to.Year()
		toMonth := int(to.Month())
		toDay := to.Day()

		totalMonths := (toYear-fromYear)*12 + (toMonth - fromMonth)
		if totalMonths > 0 && toDay < fromDay {
			totalMonths--
		} else if totalMonths < 0 && toDay > fromDay {
			totalMonths++
		}

		dur := &FEELDuration{IsYM: true}
		if totalMonths < 0 {
			dur.Neg = true
			totalMonths = -totalMonths
		}
		dur.Years = totalMonths / 12
		dur.Months = totalMonths % 12
		return dur, nil
	}).Optional("from", "to").Vararg("__extra"))
}
