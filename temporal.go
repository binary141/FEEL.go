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

// time
type FEELTime struct {
	t   time.Time
	src string
}

func (st FEELTime) Time() time.Time {
	return st.t
}

var timePatterns = []string{
	"15:04:05.999999999-07:00:00",
	"15:04:05.999999999-07:00",
	"15:04:05.999999999",
	"15:04:05-07:00:00",
	"15:04:05-07:00",
	"15:04:05@MST",
	"15:04:05",
}

func ParseTime(temporalStr string) (*FEELTime, error) {
	if atIdx := strings.Index(temporalStr, "@"); atIdx > 0 {
		timePart := temporalStr[:atIdx]
		tzName := temporalStr[atIdx+1:]
		if strings.Contains(tzName, "/") || len(tzName) > 3 {
			if loc, err := time.LoadLocation(tzName); err == nil {
				for _, pat := range []string{"15:04:05.999999999", "15:04:05"} {
					if t, err := time.ParseInLocation(pat, timePart, loc); err == nil {
						return &FEELTime{t: t, src: temporalStr}, nil
					}
				}
			}
			return nil, ErrParseTemporal
		}
	}
	for _, pat := range timePatterns {
		if t, err := time.Parse(pat, temporalStr); err == nil {
			return &FEELTime{t: t, src: temporalStr}, nil
		}
	}
	return nil, ErrParseTemporal
}

func (st FEELTime) GetAttr(name string) (interface{}, bool) {
	switch name {
	case "hour":
		return st.t.Hour(), true
	case "minute":
		return st.t.Minute(), true
	case "second":
		return st.t.Second(), true
	case "timezone":
		zoneName, _ := st.t.Zone()
		return zoneName, true
	case "timezone offset":
		_, offset := st.t.Zone()
		return offset, true
	}
	return nil, false
}

func (st FEELTime) MarshalJSON() ([]byte, error) {
	return json.Marshal(st.String())
}

func (st FEELTime) String() string {
	if st.src != "" {
		return st.src
	}
	return st.t.Format("15:04:05-07:00")
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

func (date FEELDate) GetAttr(name string) (interface{}, bool) {
	switch name {
	case "year":
		return date.t.Year(), true
	case "month":
		return date.t.Month(), true
	case "day":
		return date.t.Day(), true
	}
	return nil, false
}

func (date FEELDate) String() string {
	return date.t.Format("2006-01-02")
}

func (date FEELDate) MarshalJSON() ([]byte, error) {
	return json.Marshal(date.String())
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
	return nil, ErrParseTemporal
}

// Date and Time
type FEELDatetime struct {
	t   time.Time
	src string
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

func (sdt FEELDatetime) GetAttr(name string) (interface{}, bool) {
	switch name {
	case "year":
		return sdt.t.Year(), true
	case "month":
		return sdt.t.Month(), true
	case "day":
		return sdt.t.Day(), true
	case "hour":
		return sdt.t.Hour(), true
	case "minute":
		return sdt.t.Minute(), true
	case "second":
		return sdt.t.Second(), true
	case "timezone":
		zoneName, _ := sdt.t.Zone()
		return zoneName, true
	case "timezone offset":
		_, offset := sdt.t.Zone()
		return offset, true
	}
	return nil, false
}

func (sdt FEELDatetime) MarshalJSON() ([]byte, error) {
	return json.Marshal(sdt.String())
}

func (sdt FEELDatetime) String() string {
	if sdt.src != "" {
		return sdt.src
	}
	return sdt.t.Format("2006-01-02T15:04:05@MST")
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
		}
	}
	return &FEELDatetime{t: sdt.t.Add(dur.Duration())}
}

func (sdt *FEELDatetime) Sub(v HasTime) *FEELDuration {
	delta := sdt.t.Sub(v.Time())
	return NewFEELDuration(delta)
}

var dateTimePatterns = []string{
	"2006-01-02T15:04:05.999999999-07:00:00",
	"2006-01-02T15:04:05.999999999-07:00",
	"2006-01-02T15:04:05.999999999",
	"2006-01-02T15:04:05-07:00:00",
	"2006-01-02T15:04:05-07:00",
	"2006-01-02T15:04:05@MST",
	"2006-01-02T15:04:05",
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
		return &FEELDatetime{t: negT, src: temporalStr}, nil
	}
	// date-only: normalize to YYYY-MM-DDTHH:MM:SS
	if matched, _ := regexp.MatchString(`^\d{4}-\d{2}-\d{2}$`, temporalStr); matched {
		if t, err := time.Parse("2006-01-02", temporalStr); err == nil {
			return &FEELDatetime{t: t, src: temporalStr + "T00:00:00"}, nil
		}
	}
	// @IANA timezone with slash or long name (e.g. Etc/UTC)
	if atIdx := strings.LastIndex(temporalStr, "@"); atIdx > 10 {
		tzName := temporalStr[atIdx+1:]
		if strings.Contains(tzName, "/") || len(tzName) > 3 {
			dtPart := temporalStr[:atIdx]
			if loc, err := time.LoadLocation(tzName); err == nil {
				for _, pat := range []string{"2006-01-02T15:04:05.999999999", "2006-01-02T15:04:05"} {
					if t, err := time.ParseInLocation(pat, dtPart, loc); err == nil {
						return &FEELDatetime{t: t, src: temporalStr}, nil
					}
				}
			}
			return nil, ErrParseTemporal
		}
	}
	for _, pat := range dateTimePatterns {
		if t, err := time.Parse(pat, temporalStr); err == nil {
			return &FEELDatetime{t: t, src: temporalStr}, nil
		}
	}
	return nil, ErrParseTemporal
}

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

func (dur FEELDuration) GetAttr(name string) (interface{}, bool) {
	switch name {
	case "years":
		return dur.Years, true
	case "months":
		return dur.Months, true
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

func (dur FEELDuration) String() string {
	neg := ""
	if dur.Neg {
		neg = "-"
	}
	if dur.IsYM {
		if dur.Years == 0 && dur.Months == 0 {
			return neg + "P0Y"
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
	if dur.Days == 0 && dur.Hours == 0 && dur.Minutes == 0 && dur.Seconds == 0 && dur.SecondsFrac == "" {
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
	if dur.Seconds != 0 || dur.SecondsFrac != "" {
		sTime += fmt.Sprintf("%d%sS", dur.Seconds, dur.SecondsFrac)
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
var timeDurationPattern = regexp.MustCompile(`^(\-?)P((\d+)D)?(T((\d+)H)?((\d+)M)?((\d+)(\.\d+)?S)?)?$`)

func ParseDuration(temporalStr string) (*FEELDuration, error) {
	// parse year-month duration
	if submatches := yearmonthDurationPattern.FindStringSubmatch(temporalStr); submatches != nil {
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
			// normalize hours >= 24 into days
			dur.Days += dur.Hours / 24
			dur.Hours = dur.Hours % 24
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
			dur.SecondsFrac = submatches[11] // ".1234" or ""
		}
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

func ParseTemporalValue(temporalStr string) (interface{}, error) {
	if v, err := ParseDatetime(temporalStr); err == nil {
		return v, nil
	}

	if v, err := ParseTime(temporalStr); err == nil {
		return v, nil
	}

	if v, err := ParseDate(temporalStr); err == nil {
		return v, nil
	}

	return ParseDuration(temporalStr)
}

// builtin functions
func installDatetimeFunctions(prelude *Prelude) {
	// conversions
	prelude.Bind("date", NewNativeFunc(func(args map[string]any) (any, error) {
		fromVal, ok := args["from"]
		if !ok {
			return Null, nil
		}
		if _, isNull := fromVal.(*NullValue); isNull {
			return Null, nil
		}
		frm, ok := fromVal.(string)
		if !ok {
			return Null, nil
		}
		d, err := ParseDate(frm)
		if err != nil {
			return Null, nil
		}
		return d, nil
	}).Required("from"))

	prelude.Bind("time", wrapTyped(func(frm string) (interface{}, error) {
		return ParseTime(frm)
	}).Required("from"))

	prelude.Bind("date and time", wrapTyped(func(frm string) (interface{}, error) {
		return ParseDatetime(frm)
	}).Required("from"))

	prelude.Bind("duration", wrapTyped(func(frm string) (interface{}, error) {
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

	prelude.Bind("day of week", wrapTyped(func(v HasDate) (interface{}, error) {
		return v.Date().Weekday(), nil
	}).Required("date"))

	prelude.Bind("day of year", wrapTyped(func(v HasDate) (interface{}, error) {
		return v.Date().YearDay(), nil
	}).Required("date"))

	prelude.Bind("week of year", wrapTyped(func(v HasDate) (interface{}, error) {
		_, week := v.Date().ISOWeek()
		return week, nil
	}).Required("date"))

	prelude.Bind("month of year", wrapTyped(func(v HasDate) (interface{}, error) {
		return v.Date().Month(), nil
	}).Required("date"))

	prelude.Bind("abs", wrapTyped(func(dur *FEELDuration) (interface{}, error) {
		newDur := *dur
		newDur.Neg = false
		return newDur, nil
	}).Required("dur"))

	// refs https://docs.camunda.io/docs/components/modeler/feel/builtin-functions/feel-built-in-functions-temporal/#last-day-of-monthdate
	prelude.Bind("last day of month", wrapTyped(func(v HasDate) (interface{}, error) {
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
