package widget

import (
	"testing"

	"github.com/joncooper/imagine-tui/internal/dom"
)

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Fatal("NewRegistry returned nil")
	}
}

func TestRegistry_Register_and_Create(t *testing.T) {
	r := NewRegistry()
	r.Register(dom.TypeText, func() Widget { return &stubWidget{typeName: "text"} })

	w, err := r.Create(dom.TypeText)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sw, ok := w.(*stubWidget)
	if !ok {
		t.Fatal("expected *stubWidget")
	}
	if sw.typeName != "text" {
		t.Errorf("typeName = %q, want %q", sw.typeName, "text")
	}
}

func TestRegistry_Create_UnknownType(t *testing.T) {
	r := NewRegistry()
	_, err := r.Create(dom.TypeText)
	if err == nil {
		t.Fatal("expected error for unregistered type")
	}
}

func TestRegistry_Create_ReturnsNewInstances(t *testing.T) {
	r := NewRegistry()
	r.Register(dom.TypeText, func() Widget { return &stubWidget{} })

	w1, _ := r.Create(dom.TypeText)
	w2, _ := r.Create(dom.TypeText)
	if w1 == w2 {
		t.Error("expected distinct instances from each Create call")
	}
}

func TestRegistry_Register_Overwrite(t *testing.T) {
	r := NewRegistry()
	r.Register(dom.TypeText, func() Widget { return &stubWidget{typeName: "old"} })
	r.Register(dom.TypeText, func() Widget { return &stubWidget{typeName: "new"} })

	w, _ := r.Create(dom.TypeText)
	sw := w.(*stubWidget)
	if sw.typeName != "new" {
		t.Errorf("typeName = %q, want %q after overwrite", sw.typeName, "new")
	}
}

func TestRegistry_Has(t *testing.T) {
	r := NewRegistry()
	if r.Has(dom.TypeText) {
		t.Error("expected Has to return false for unregistered type")
	}
	r.Register(dom.TypeText, func() Widget { return &stubWidget{} })
	if !r.Has(dom.TypeText) {
		t.Error("expected Has to return true after registration")
	}
}
