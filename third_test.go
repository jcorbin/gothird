package main

import "testing"

func Test_Third(t *testing.T) {
	vmTest("setup").
		withInputWriter(thirdKernel).
		withTestHexOutput().
		withMemAt(255, 1 /* TRON */).
		run(t)
}
