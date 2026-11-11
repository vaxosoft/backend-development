// Dates, layouts, durations, time zones and the monotonic clock.
package main

import (
	"fmt"
	"time"
	_ "time/tzdata" // embed the zone database so LoadLocation works on any machine
)

func main() {
	// Layouts are written with the reference time Mon Jan 2 15:04:05 MST 2006.
	t := time.Date(2030, time.January, 31, 9, 30, 0, 0, time.UTC)
	fmt.Println(t.Format(time.RFC3339))
	fmt.Println(t.Format("2006-01-02"))
	fmt.Println(t.Format("Mon 02 Jan 2006, 3:04PM"))

	parsed, err := time.Parse(time.DateOnly, "2030-02-30")
	fmt.Println("parse invalid date:", parsed.IsZero(), err)

	// Durations are int64 nanoseconds, not calendar units.
	d := 90*time.Minute + 30*time.Second
	fmt.Println(d, d.Hours(), d.Round(time.Hour), d.Truncate(time.Hour))
	fmt.Println(t.Add(d).Format(time.TimeOnly), t.Add(d).Sub(t) == d)

	// AddDate works on the calendar and normalises overflow: Jan 31 + 1 month = Mar 3.
	fmt.Println(t.AddDate(0, 1, 0).Format(time.DateOnly))
	fmt.Println(t.Truncate(24 * time.Hour).Format(time.RFC3339))

	// The same instant in different zones; the offset depends on the date (DST).
	ny, err := time.LoadLocation("America/New_York")
	if err != nil {
		panic(err)
	}
	winter := time.Date(2030, time.January, 15, 12, 0, 0, 0, ny)
	summer := time.Date(2030, time.July, 15, 12, 0, 0, 0, ny)
	fmt.Println(winter.Format(time.RFC3339), summer.Format(time.RFC3339))
	fmt.Println(winter.UTC().Format(time.RFC3339), winter.Equal(winter.UTC()), winter == winter.UTC())

	// Wall-clock arithmetic across a DST change: "tomorrow" is not always +24h.
	beforeShift := time.Date(2030, time.March, 9, 12, 0, 0, 0, ny)
	fmt.Println(beforeShift.AddDate(0, 0, 1).Sub(beforeShift), beforeShift.Add(24*time.Hour).Format(time.RFC3339))

	// time.Now carries a monotonic reading, so Since/Sub ignore wall-clock jumps.
	start := time.Now()
	fmt.Println("has monotonic reading:", start != start.Round(0))
	fmt.Println("elapsed is non-negative:", time.Since(start) >= 0)

	// Compare instants with Equal/Before/After, never with ==.
	a := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	b := a.In(time.FixedZone("UTC+4", 4*3600))
	fmt.Println(a.Equal(b), a == b, a.Before(b.Add(time.Nanosecond)))

	// Wire format: accept any offset, store and emit RFC 3339 UTC (the Task Tracker rule).
	in, err := time.Parse(time.RFC3339, "2030-01-01T13:00:00+04:00")
	if err != nil {
		panic(err)
	}
	fmt.Println(in.UTC().Format(time.RFC3339))
}
