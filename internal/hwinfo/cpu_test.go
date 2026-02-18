package hwinfo

import (
	"testing"
)

func TestCPUModel_NonEmpty(t *testing.T) {
	model := CPUModel()
	if model == "" {
		t.Fatal("CPUModel() returned an empty string; it should always return something")
	}
	t.Logf("detected CPU model: %s", model)
}

func TestCPUModel_Deterministic(t *testing.T) {
	a := CPUModel()
	b := CPUModel()
	if a != b {
		t.Errorf("CPUModel() returned different values on consecutive calls: %q vs %q", a, b)
	}
}
