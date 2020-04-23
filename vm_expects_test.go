package main

// @generated from vm_test.go

//go:generate go run scripts/gen_vm_expects.go -- vm_test.go vm_expects_test.go

func expectVMError(err error) func(vmTestCase) vmTestCase {
	return func(vmt vmTestCase) vmTestCase {
		return vmt.expectError(err)
	}
}

func expectVMProg(prog uint) func(vmTestCase) vmTestCase {
	return func(vmt vmTestCase) vmTestCase {
		return vmt.expectProg(prog)
	}
}

func expectVMLast(last uint) func(vmTestCase) vmTestCase {
	return func(vmt vmTestCase) vmTestCase {
		return vmt.expectLast(last)
	}
}

func expectVMStack(values ...int) func(vmTestCase) vmTestCase {
	return func(vmt vmTestCase) vmTestCase {
		return vmt.expectStack(values...)
	}
}

func expectVMRStack(values ...int) func(vmTestCase) vmTestCase {
	return func(vmt vmTestCase) vmTestCase {
		return vmt.expectRStack(values...)
	}
}

func expectVMString(id uint, s string) func(vmTestCase) vmTestCase {
	return func(vmt vmTestCase) vmTestCase {
		return vmt.expectString(id, s)
	}
}

func expectVMMemAt(addr uint, values ...int) func(vmTestCase) vmTestCase {
	return func(vmt vmTestCase) vmTestCase {
		return vmt.expectMemAt(addr, values...)
	}
}

func expectVMWord(addr uint, name string, code ...int) func(vmTestCase) vmTestCase {
	return func(vmt vmTestCase) vmTestCase {
		return vmt.expectWord(addr, name, code...)
	}
}

func expectVMH(value int) func(vmTestCase) vmTestCase {
	return func(vmt vmTestCase) vmTestCase {
		return vmt.expectH(value)
	}
}

func expectVMR(value int) func(vmTestCase) vmTestCase {
	return func(vmt vmTestCase) vmTestCase {
		return vmt.expectR(value)
	}
}

func expectVMOutput(output string) func(vmTestCase) vmTestCase {
	return func(vmt vmTestCase) vmTestCase {
		return vmt.expectOutput(output)
	}
}

func expectVMDump(dump string) func(vmTestCase) vmTestCase {
	return func(vmt vmTestCase) vmTestCase {
		return vmt.expectDump(dump)
	}
}
