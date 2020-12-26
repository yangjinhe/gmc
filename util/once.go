package gutil

import (
	"crypto/rand"
	"fmt"
	"io"
	"sync"
)

var onceDataMap = sync.Map{}
var onceDoDataMap = sync.Map{}

func OnceDo(uniqueKey string, f func()) {
	once, _ := onceDoDataMap.LoadOrStore(uniqueKey, &sync.Once{})
	once.(*sync.Once).Do(f)
	return
}

func LoadOnce(uniqueKey string) *sync.Once {
	if uniqueKey == "" {
		key := make([]byte, 16)
		io.ReadFull(rand.Reader, key)
		uniqueKey = fmt.Sprintf("%x", key)
	}
	once, _ := onceDataMap.LoadOrStore(uniqueKey, &sync.Once{})
	return once.(*sync.Once)
}

func RemoveOnce(uniqueKey string) {
	onceDataMap.Delete(uniqueKey)
	return
}
