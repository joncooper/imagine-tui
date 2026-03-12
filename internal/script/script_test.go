package script_test

import (
	"testing"

	"github.com/dop251/goja"
)

func TestGojaImport(t *testing.T) {
	// Verify goja dependency is available and can execute JS.
	vm := goja.New()
	v, err := vm.RunString("1 + 2")
	if err != nil {
		t.Fatalf("goja RunString failed: %v", err)
	}
	if v.Export().(int64) != 3 {
		t.Fatalf("expected 3, got %v", v.Export())
	}
	t.Log("goja import OK")
}
