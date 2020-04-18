package main

import "testing"

func Test_Third(t *testing.T) {
	vmTest("setup").withOptions(
		withMemLayout(16, 80),
		withInputWriter(thirdKernel),
	).withTestHexOutput().withTestLog().run(t)
}
