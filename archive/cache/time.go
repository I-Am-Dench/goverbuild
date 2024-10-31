package cache

import (
	"fmt"
	"math"
	"time"
)

var (
	microToNano int64 = 1000
)

// Both the client and the original patcher round the
// nanosecond value to microseconds using the RoundToEven function:
// https://en.wikipedia.org/wiki/Rounding#Rounding_half_to_even
func RoundNanoseconds(n int) int64 {
	return int64(math.RoundToEven(float64(n) / float64(microToNano)))
}

func RoundTime(t time.Time) time.Time {
	return time.Unix(t.Unix(), RoundNanoseconds(t.Nanosecond())*microToNano)
}

func FormatTime(t time.Time) string {
	return fmt.Sprintf("%d.%06d", t.Unix(), RoundNanoseconds(t.Nanosecond()))
}
