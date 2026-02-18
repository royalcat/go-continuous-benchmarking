package hwinfo

import (
	"fmt"
	"runtime"

	"github.com/klauspost/cpuid/v2"
)

// CPUModel returns a human-readable string identifying the CPU of the
// current machine.  It uses klauspost/cpuid which works on x86, ARM
// and other architectures.
//
// If the library cannot determine a brand name (common on some ARM
// boards), a fallback string built from GOARCH and the available
// identifiers is returned.  The result is never empty.
func CPUModel() string {
	brand := cpuid.CPU.BrandName
	if brand != "" {
		return brand
	}

	// Fallback: build something useful from what cpuid does expose.
	name := cpuid.CPU.VendorString
	if name == "" {
		name = cpuid.CPU.VendorID.String()
	}
	if name == "" || name == "Unknown" {
		name = runtime.GOARCH
	}

	cores := cpuid.CPU.PhysicalCores
	if cores > 0 {
		return fmt.Sprintf("%s (%d cores)", name, cores)
	}

	return name
}
