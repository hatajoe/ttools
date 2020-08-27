package gen

import "sync/atomic"

var (
	id int64
)

// ID generates a unique number value which is safe to call in concurrently
func ID() int64 {
	return atomic.AddInt64(&id, 1)
}
