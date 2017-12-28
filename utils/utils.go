package utils

import (
	"crypto/rand"
	"time"
)

// TickToHighResTimer provides a method to transform requestAnimationFrame
// clock elapsed time into a appropriate time.Duration
func TickToHighResTimer(ms float64) time.Duration {
	return time.Duration(ms * float64(time.Millisecond))
}

// RandString generates a set of random numbers of a set length
func RandString(n int) string {
	const alphanum = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	var bytes = make([]byte, n)
	rand.Read(bytes)
	for i, b := range bytes {
		bytes[i] = alphanum[b%byte(len(alphanum))]
	}
	return string(bytes)
}

// Times using the provided count, runs the function (n-1) number of times, since
// it starts from zero.
func Times(n int, fn func(int)) {
	for i := 0; i < n; i++ {
		fn(i + 1)
	}
}
