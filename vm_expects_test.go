package main

import (
	"io"
	"time"
)

// @generated from vm_test.go

//go:generate go run scripts/gen_vm_expects.go -- vm_test.go vm_expects_test.go

func withVMOptions(opts ...VMOption) func(vmTestCase) vmTestCase {
	return func(vmt vmTestCase) vmTestCase {
		return vmt.withOptions(opts...)
	}
}

func withVMProg(prog uint) func(vmTestCase) vmTestCase {
	return func(vmt vmTestCase) vmTestCase {
		return vmt.withProg(prog)
	}
}

func withVMLast(last uint) func(vmTestCase) vmTestCase {
	return func(vmt vmTestCase) vmTestCase {
		return vmt.withLast(last)
	}
}

func withVMStack(values ...int) func(vmTestCase) vmTestCase {
	return func(vmt vmTestCase) vmTestCase {
		return vmt.withStack(values...)
	}
}

func withVMStrings(idStringPairs ...interface{}) func(vmTestCase) vmTestCase {
	return func(vmt vmTestCase) vmTestCase {
		return vmt.withStrings(idStringPairs...)
	}
}

func withVMString(id uint, s string) func(vmTestCase) vmTestCase {
	return func(vmt vmTestCase) vmTestCase {
		return vmt.withString(id, s)
	}
}

func withVMMemAt(addr uint, values ...int) func(vmTestCase) vmTestCase {
	return func(vmt vmTestCase) vmTestCase {
		return vmt.withMemAt(addr, values...)
	}
}

func withVMH(val int) func(vmTestCase) vmTestCase {
	return func(vmt vmTestCase) vmTestCase {
		return vmt.withH(val)
	}
}

func withVMR(val int) func(vmTestCase) vmTestCase {
	return func(vmt vmTestCase) vmTestCase {
		return vmt.withR(val)
	}
}

func withVMPageSize(pageSize uint) func(vmTestCase) vmTestCase {
	return func(vmt vmTestCase) vmTestCase {
		return vmt.withPageSize(pageSize)
	}
}

func withVMRetBase(addr uint, values ...int) func(vmTestCase) vmTestCase {
	return func(vmt vmTestCase) vmTestCase {
		return vmt.withRetBase(addr, values...)
	}
}

func withVMMemBase(addr uint, values ...int) func(vmTestCase) vmTestCase {
	return func(vmt vmTestCase) vmTestCase {
		return vmt.withMemBase(addr, values...)
	}
}

func withVMMemLimit(limit uint) func(vmTestCase) vmTestCase {
	return func(vmt vmTestCase) vmTestCase {
		return vmt.withMemLimit(limit)
	}
}

func withVMInput(input string) func(vmTestCase) vmTestCase {
	return func(vmt vmTestCase) vmTestCase {
		return vmt.withInput(input)
	}
}

func withVMNamedInput(name string, input string) func(vmTestCase) vmTestCase {
	return func(vmt vmTestCase) vmTestCase {
		return vmt.withNamedInput(name, input)
	}
}

func withVMInputWriter(w io.WriterTo) func(vmTestCase) vmTestCase {
	return func(vmt vmTestCase) vmTestCase {
		return vmt.withInputWriter(w)
	}
}

func withVMTimeout(timeout time.Duration) func(vmTestCase) vmTestCase {
	return func(vmt vmTestCase) vmTestCase {
		return vmt.withTimeout(timeout)
	}
}

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
