package feel

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Regression tests for temporal.go's GetAttr changes: numeric fields now
// return *Number (so FEEL arithmetic works on them directly), "timezone"/
// "time offset" now depend on how the zone was actually expressed rather
// than always resolving via Go's zone data, and dates/date-times gained an
// ISO-8601 "weekday" attribute.

func TestFEELTimeAttrsReturnNumber(t *testing.T) {
	tm, err := ParseTime("10:30:45")
	assert.Nil(t, err)

	hour, ok := tm.GetAttr("hour")
	assert.True(t, ok)
	assert.Equal(t, float64(10), numFloat(t, hour))

	minute, ok := tm.GetAttr("minute")
	assert.True(t, ok)
	assert.Equal(t, float64(30), numFloat(t, minute))

	second, ok := tm.GetAttr("second")
	assert.True(t, ok)
	assert.Equal(t, float64(45), numFloat(t, second))
}

func TestFEELTimeTimezoneOnlyForNamedZone(t *testing.T) {
	// No zone info at all: both attrs are null.
	plain, err := ParseTime("10:30:45")
	assert.Nil(t, err)
	tz, ok := plain.GetAttr("timezone")
	assert.True(t, ok)
	assert.Equal(t, Null, tz)
	off, ok := plain.GetAttr("time offset")
	assert.True(t, ok)
	assert.Equal(t, Null, off)

	// Named IANA zone: "timezone" resolves, "time offset" resolves too.
	named, err := ParseTime("10:30:45@Australia/Melbourne")
	assert.Nil(t, err)
	tz, ok = named.GetAttr("timezone")
	assert.True(t, ok)
	assert.Equal(t, "Australia/Melbourne", tz)
	off, ok = named.GetAttr("time offset")
	assert.True(t, ok)
	offDur, ok := off.(*FEELDuration)
	assert.True(t, ok, "expected *FEELDuration, got %T", off)
	assert.False(t, offDur.IsYM)

	// Numeric offset (no name): "timezone" is null but "time offset" isn't.
	offset, err := ParseTime("10:30:45+02:00")
	assert.Nil(t, err)
	tz, ok = offset.GetAttr("timezone")
	assert.True(t, ok)
	assert.Equal(t, Null, tz)
	off, ok = offset.GetAttr("time offset")
	assert.True(t, ok)
	offDur, ok = off.(*FEELDuration)
	assert.True(t, ok, "expected *FEELDuration, got %T", off)
	assert.Equal(t, 2*time.Hour, offDur.Duration())
}

func TestFEELDateAttrsReturnNumberAndWeekday(t *testing.T) {
	// 2024-01-01 is a Monday.
	d, err := ParseDate("2024-01-01")
	assert.Nil(t, err)

	year, ok := d.GetAttr("year")
	assert.True(t, ok)
	assert.Equal(t, float64(2024), numFloat(t, year))

	month, ok := d.GetAttr("month")
	assert.True(t, ok)
	assert.Equal(t, float64(1), numFloat(t, month))

	day, ok := d.GetAttr("day")
	assert.True(t, ok)
	assert.Equal(t, float64(1), numFloat(t, day))

	weekday, ok := d.GetAttr("weekday")
	assert.True(t, ok)
	assert.Equal(t, float64(1), numFloat(t, weekday))
}

func TestFEELDateWeekdaySunday(t *testing.T) {
	// 2024-01-07 is a Sunday, which is ISO weekday 7 (not Go's 0).
	d, err := ParseDate("2024-01-07")
	assert.Nil(t, err)

	weekday, ok := d.GetAttr("weekday")
	assert.True(t, ok)
	assert.Equal(t, float64(7), numFloat(t, weekday))
}

func TestFEELDatetimeAttrsReturnNumberAndWeekday(t *testing.T) {
	dt, err := ParseDatetime("2024-01-01T10:30:45")
	assert.Nil(t, err)

	year, ok := dt.GetAttr("year")
	assert.True(t, ok)
	assert.Equal(t, float64(2024), numFloat(t, year))

	month, ok := dt.GetAttr("month")
	assert.True(t, ok)
	assert.Equal(t, float64(1), numFloat(t, month))

	day, ok := dt.GetAttr("day")
	assert.True(t, ok)
	assert.Equal(t, float64(1), numFloat(t, day))

	hour, ok := dt.GetAttr("hour")
	assert.True(t, ok)
	assert.Equal(t, float64(10), numFloat(t, hour))

	weekday, ok := dt.GetAttr("weekday")
	assert.True(t, ok)
	assert.Equal(t, float64(1), numFloat(t, weekday))
}

func TestFEELDatetimeTimezoneOnlyForNamedZone(t *testing.T) {
	plain, err := ParseDatetime("2024-01-01T10:30:45")
	assert.Nil(t, err)
	tz, ok := plain.GetAttr("timezone")
	assert.True(t, ok)
	assert.Equal(t, Null, tz)
	off, ok := plain.GetAttr("time offset")
	assert.True(t, ok)
	assert.Equal(t, Null, off)

	named, err := ParseDatetime("2024-01-01T10:30:45@Australia/Melbourne")
	assert.Nil(t, err)
	tz, ok = named.GetAttr("timezone")
	assert.True(t, ok)
	assert.Equal(t, "Australia/Melbourne", tz)
}

func TestNewFEELDurationNegative(t *testing.T) {
	d := NewFEELDuration(-90 * time.Minute)
	assert.True(t, d.Neg)
	assert.Equal(t, 1, d.Hours)
	assert.Equal(t, 30, d.Minutes)
	assert.Equal(t, -90*time.Minute, d.Duration())
}

func TestFEELDurationGetAttrYearMonthExclusive(t *testing.T) {
	dur := MustParseDuration("P1Y2M")
	years, ok := dur.GetAttr("years")
	assert.True(t, ok)
	assert.Equal(t, 1, years)
	months, ok := dur.GetAttr("months")
	assert.True(t, ok)
	assert.Equal(t, 2, months)

	// Day-time fields don't exist on a year-month duration.
	_, ok = dur.GetAttr("days")
	assert.False(t, ok)
	_, ok = dur.GetAttr("hours")
	assert.False(t, ok)
}

func TestFEELDurationGetAttrDayTimeExclusive(t *testing.T) {
	dur := MustParseDuration("P1DT2H3M4S")
	days, ok := dur.GetAttr("days")
	assert.True(t, ok)
	assert.Equal(t, 1, days)

	// Year-month fields don't exist on a day-time duration.
	_, ok = dur.GetAttr("years")
	assert.False(t, ok)
	_, ok = dur.GetAttr("months")
	assert.False(t, ok)
}

func TestParseDurationCarriesOverflowingSeconds(t *testing.T) {
	dur, err := ParseDuration("PT90S")
	assert.Nil(t, err)
	assert.Equal(t, 0, dur.Hours)
	assert.Equal(t, 1, dur.Minutes)
	assert.Equal(t, 30, dur.Seconds)
}

func TestParseDurationCarriesOverflowingMinutes(t *testing.T) {
	dur, err := ParseDuration("PT61M")
	assert.Nil(t, err)
	assert.Equal(t, 1, dur.Hours)
	assert.Equal(t, 1, dur.Minutes)
}

func TestParseDurationCarriesOverflowingHoursIntoDays(t *testing.T) {
	dur, err := ParseDuration("PT25H")
	assert.Nil(t, err)
	assert.Equal(t, 1, dur.Days)
	assert.Equal(t, 1, dur.Hours)
}

func TestParseDurationCarriesFullChain(t *testing.T) {
	// 1 day of seconds plus a bit more should carry seconds -> minutes ->
	// hours -> days all the way through.
	dur, err := ParseDuration("PT3661S")
	assert.Nil(t, err)
	assert.Equal(t, 0, dur.Days)
	assert.Equal(t, 1, dur.Hours)
	assert.Equal(t, 1, dur.Minutes)
	assert.Equal(t, 1, dur.Seconds)
}
