package gen

import (
	"sync"
	"testing"
)

func Test_ID(t *testing.T) {
	expected := 50000

	wg := &sync.WaitGroup{}
	for i := 0; i < expected-1; i++ {
		wg.Add(1)
		go func() {
			ID()
			wg.Done()
		}()
	}
	wg.Wait()

	if id := ID(); int64(expected) != id {
		t.Errorf("ID returns %d was expected, but %d was returned", expected, id)
	}
}
