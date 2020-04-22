package main

import (
	"testing"
	"time"
)

func Test_Third(t *testing.T) {
	vmTest("setup").
		withInputWriter(thirdKernel).
		withTestHexOutput().
		withTimeout(10*time.Second).
		withMemAt(255, 1 /* TRON */).
		run(t)
}
