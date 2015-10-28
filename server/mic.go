package main

import (
	"crypto/sha512"

	"github.com/cocoonlife/goalsa"
)

type mic struct {
	lastSample [8000]byte
}

func min(a int, b int) int {
	if a <= b {
		return a
	}
	return b
}

func (m *mic) Read(p []byte) (n int, err error) {
	dev, err := alsa.NewCaptureDevice("default", 1, alsa.FormatU8, 8000, alsa.BufferParams{})
	if err != nil {
		return
	}
	b1 := make([]int8, 8000)
	_, err = dev.Read(b1)
	dev.Close()
	if err != nil {
		return
	}
	var b2 [8000]byte
	for i := 0; i < len(b1); i++ {
		b2[i] = byte(b1[i])
	}
	c := sha512.Sum512(b2[:])
	for n = 0; n < min(len(b1), len(p)); n++ {
		p[n] = c[n]
	}
	m.lastSample = b2
	return
}
