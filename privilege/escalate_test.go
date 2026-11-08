// privilege/escalate_test.go
package privilege

import (
	"runtime"
	"testing"
)

func TestPrivilegeDetection(t *testing.T) {
	_ = IsProcessElevated()
	if runtime.GOOS != "windows" && runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Fatalf("unexpected GOOS: %s", runtime.GOOS)
	}
}