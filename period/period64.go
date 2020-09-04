package period

import (
	"fmt"
	"math"
	"strings"
)

// used for stages in arithmetic
type period64 struct {
	// always positive values
	years, months, days, hours, minutes, seconds int64
	// fraction applies to just one of the fields
	fraction int8
	fpart    designator
	// true if the period is negative
	neg bool
	// the original input string
	input string
}

func p64Of(cym, cd, chms int64, neg bool) *period64 {
	p64 := &period64{
		months:  cym / 100,
		days:    cd / 100,
		seconds: chms / 100,
		neg:     neg,
	}

	ymf := cym % 100
	if ymf != 0 {
		p64.fraction = int8(ymf)
		p64.fpart = Month
	}

	df := cd % 100
	if df != 0 {
		p64.fraction = int8(df)
		p64.fpart = Day
	}

	sf := chms % 100
	if sf != 0 {
		p64.fraction = int8(sf)
		p64.fpart = Second
	}

	return p64.normalise64(true)
}

func (period Period) toPeriod64(input string) *period64 {
	if period.IsNegative() {
		return &period64{
			years: int64(-period.years), months: int64(-period.months), days: int64(-period.days),
			hours: int64(-period.hours), minutes: int64(-period.minutes), seconds: int64(-period.seconds),
			fraction: -period.fraction,
			fpart:    period.fpart,
			input:    input,
			neg:      true,
		}
	}
	return &period64{
		years: int64(period.years), months: int64(period.months), days: int64(period.days),
		hours: int64(period.hours), minutes: int64(period.minutes), seconds: int64(period.seconds),
		fraction: period.fraction,
		fpart:    period.fpart,
		input:    input,
	}
}

func (p64 *period64) toPeriod() (Period, error) {
	var f []string
	if p64.years > math.MaxInt16 {
		f = append(f, "years")
	}
	if p64.months > math.MaxInt16 {
		f = append(f, "months")
	}
	if p64.days > math.MaxInt16 {
		f = append(f, "days")
	}
	if p64.hours > math.MaxInt16 {
		f = append(f, "hours")
	}
	if p64.minutes > math.MaxInt16 {
		f = append(f, "minutes")
	}
	if p64.seconds > math.MaxInt16 {
		f = append(f, "seconds")
	}
	if p64.fraction > 99 { // the fraction represents two decimal places
		f = append(f, "fraction")
	}

	if len(f) > 0 {
		if p64.input == "" {
			p64.input = p64.String()
		}
		return Period{}, fmt.Errorf("%s: integer overflow occurred in %s", p64.input, strings.Join(f, ","))
	}

	if p64.neg {
		return Period{
			years: int16(-p64.years), months: int16(-p64.months), days: int16(-p64.days),
			hours: int16(-p64.hours), minutes: int16(-p64.minutes), seconds: int16(-p64.seconds),
			fraction: -p64.fraction,
			fpart:    p64.fpart,
		}, nil
	}

	return Period{
		years: int16(p64.years), months: int16(p64.months), days: int16(p64.days),
		hours: int16(p64.hours), minutes: int16(p64.minutes), seconds: int16(p64.seconds),
		fraction: p64.fraction,
		fpart:    p64.fpart,
	}, nil
}

// normalise64 operates on values in which all fields are positive
func (p64 *period64) normalise64(precise bool) *period64 {
	return p64.rippleUp(precise).
		reduceYearsFraction().
		reduceDaysFraction(precise).
		reduceHoursFraction().
		reduceMinutesFraction()
}

func (p64 *period64) rippleUp(precise bool) *period64 {
	hms := (p64.hours * 3600) + (p64.minutes * 60) + p64.seconds

	if hms < 0 {
		dd := (hms / 86400) - 1
		p64.days += dd
		hms -= dd * 86400
	}

	p64.hours = hms / 3600
	p64.minutes = (hms / 60) - (p64.hours * 60)
	p64.seconds = hms % 60

	if !precise || p64.hours > math.MaxInt16 {
		p64.days += p64.hours / 24
		p64.hours %= 24
		if p64.hours < 0 {
			p64.hours = -p64.hours
		}
	}

	// this section can introduce small arithmetic errors so
	// it is only used prevent overflow
	if p64.days > math.MaxInt16 || p64.days < 0 {
		totalHours := float64((p64.days * 24) + p64.hours)
		deltaMonthsF := totalHours / hoursPerMonthF
		deltaMonths, remMonthsF := math.Modf(deltaMonthsF)
		daysF := remMonthsF * daysPerMonthF
		days, remDays := math.Modf(daysF)
		const iota = 1.0 / 360000 // reduces unwanted rounding-down
		hoursF := (remDays * 24) + iota
		hours, remHours := math.Modf(hoursF)

		p64.months += int64(deltaMonths)
		p64.days = int64(days)
		p64.hours = int64(hours)
		p64.minutes += int64(remHours * 60)

		if p64.hours >= 24 {
			p64.days += p64.hours / 24
			p64.hours %= 24
		}
	}

	if p64.months != 0 {
		p64.years += p64.months / 12
		p64.months %= 12
	}

	return p64
}

func (p64 *period64) reduceYearsFraction() *period64 {
	if p64.fpart == Year {
		centiMonths := 12 * int64(p64.fraction)
		monthFraction := centiMonths % 100
		if monthFraction == 0 {
			p64.months += centiMonths / 100
			p64.fraction = 0
			p64.fpart = NoFraction
		}
	}

	return p64
}

func (p64 *period64) reduceDaysFraction(precise bool) *period64 {
	if !precise && p64.fpart == Day {
		centiHours := 24 * int64(p64.fraction)
		hourFraction := centiHours % 100
		if hourFraction == 0 {
			p64.hours += centiHours / 100
			p64.fraction = 0
			p64.fpart = NoFraction
		}
	}

	return p64
}

func (p64 *period64) reduceMonthsFraction(precise bool) *period64 {
	if !precise && p64.fpart == Month {
		centiDays := (daysPerMonthE6 * int64(p64.fraction)) / oneE6
		dayFraction := centiDays % 100
		if dayFraction == 0 {
			p64.days += centiDays / 100
			p64.fraction = 0
			p64.fpart = NoFraction
		}
	}

	return p64
}

func (p64 *period64) reduceHoursFraction() *period64 {
	if p64.fpart == Hour {
		centiMinutes := 60 * int64(p64.fraction)
		minuteFraction := centiMinutes % 100
		if minuteFraction == 0 {
			p64.minutes += centiMinutes / 100
			p64.fraction = 0
			p64.fpart = NoFraction
		}
	}

	return p64
}

func (p64 *period64) reduceMinutesFraction() *period64 {
	if p64.fpart == Minute {
		centiSeconds := 60 * int64(p64.fraction)
		secondFraction := centiSeconds % 100
		if secondFraction == 0 {
			p64.seconds += centiSeconds / 100
			p64.fraction = 0
			p64.fpart = NoFraction
		}
	}

	return p64
}
