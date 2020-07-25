package main

import (
	"testing"
	"time"
)

func TestEncodeDecode(t *testing.T) {
	data := []byte("Hello World")
	_, data = encodeTime(data, 1*time.Minute)
	canary := []byte{0xde, 0xad}
	if data[0] != canary[0] || data[1] != canary[1] {
		t.Error("Malformed data")
	}
}
