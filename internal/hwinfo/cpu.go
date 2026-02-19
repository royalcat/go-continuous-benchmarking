package hwinfo

import (
	"context"
	"fmt"
	"runtime"

	"github.com/shirou/gopsutil/v4/cpu"
)

// CPUModel returns a human-readable string identifying the CPU of the
// current machine.  It uses gopsutil which works on Linux, FreeBSD,
// OpenBSD, macOS, Windows, Solaris, and AIX.
//
// If the library cannot determine a model name, a fallback string
// built from GOARCH and the available identifiers is returned.
// The result is never empty.
func CPUModel() string {
	infos, err := cpu.Info()
	if err == nil && len(infos) > 0 {
		if infos[0].ModelName != "" {
			return infos[0].ModelName
		}

		// Fallback: build something useful from what gopsutil does expose.
		name := infos[0].VendorID
		if name == "" {
			name = runtime.GOARCH
		}

		cores, _ := cpu.Counts(true)
		if cores > 0 {
			return fmt.Sprintf("%s (%d cores)", name, cores)
		}

		return name
	}

	// Last resort fallback when gopsutil fails entirely.
	cores, _ := cpu.CountsWithContext(context.Background(), true)
	if cores > 0 {
		return fmt.Sprintf("%s (%d cores)", runtime.GOARCH, cores)
	}

	return runtime.GOARCH
}
