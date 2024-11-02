package cache

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

var (
	secondsToNano int64 = 1e9
	microToNano   int64 = 1000
)

// Attempting to extract the fractional component from the time as
// a float just causes the result to contain the original, accurate
// nanosecond value. But since we need the imprecise version, we
// extract the fractional component as a string, and then round that value.
func roundNanoseconds(seconds int64, nanoseconds int) int64 {
	f := float64(seconds) + (float64(nanoseconds) / float64(secondsToNano))
	s := strconv.FormatFloat(f, 'f', 7, 64)
	_, frac, _ := strings.Cut(s, ".")
	unrounded, _ := strconv.ParseFloat(frac, 64)
	return int64(math.Round(unrounded / 10))
}

// Both the client and the original patcher treat the modification time
// as a floating point number, and then rounding to 6 decimal places.
//
// Because of imprecisions within floating point numbers, the original patch (and therefore client)
// expect a slightly incorrect nanoseconds value, thus causing a typically off-by-one rounding error.
//
// For example, if a file has a modification time of 1729218809s + 315336400ns,
// the correctly rounded nanoscond value should be 315336000ns (1729218809.315336).
// Instead, the client considers the floating point time as 1729218809.3153365s,
// thus causing it to round the nanoseconds to 315337000ns (1729218809.315337) instead.
func RoundTime(t time.Time) time.Time {
	return time.Unix(t.Unix(), roundNanoseconds(t.Unix(), t.Nanosecond())*microToNano)
}

func FormatTime(t time.Time) string {
	return fmt.Sprintf("%d.%06d", t.Unix(), roundNanoseconds(t.Unix(), t.Nanosecond()))
}
