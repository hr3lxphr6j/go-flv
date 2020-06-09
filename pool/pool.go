package pool

import (
	"bytes"
	"sync"
)

var pool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

func GetBuffer() *bytes.Buffer {
	return pool.Get().(*bytes.Buffer)
}

func PutBuffer(buffer *bytes.Buffer) {
	if buffer == nil {
		return
	}
	buffer.Reset()
	pool.Put(buffer)
}
