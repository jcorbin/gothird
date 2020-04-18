package main

import "testing"

func Test_Third(t *testing.T) {
	vmTest("setup").withRetBase(16).withMemBase(80).withInputWriter(thirdKernel).withTestHexOutput().withTestLog().run(t)
}
