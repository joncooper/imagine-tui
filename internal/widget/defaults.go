package widget

import "github.com/joncooper/imagine-tui/internal/dom"

// DefaultRegistry returns a registry pre-populated with all implemented widget types.
func DefaultRegistry() *Registry {
	r := NewRegistry()
	r.Register(dom.TypeContainer, func() Widget { return &ContainerWidget{} })
	r.Register(dom.TypeText, func() Widget { return &TextWidget{} })
	r.Register(dom.TypeInput, func() Widget { return &InputWidget{} })
	r.Register(dom.TypeTextarea, func() Widget { return &TextareaWidget{} })
	r.Register(dom.TypeSelect, func() Widget { return &SelectWidget{} })
	r.Register(dom.TypeButton, func() Widget { return &ButtonWidget{} })
	r.Register(dom.TypeTable, func() Widget { return &TableWidget{} })
	r.Register(dom.TypeList, func() Widget { return &ListWidget{} })
	r.Register(dom.TypeDiff, func() Widget { return &DiffWidget{} })
	r.Register(dom.TypeLog, func() Widget { return &LogWidget{} })
	r.Register(dom.TypeCode, func() Widget { return &CodeWidget{} })
	r.Register(dom.TypeProgress, func() Widget { return &ProgressWidget{} })
	r.Register(dom.TypeSpinner, func() Widget { return &SpinnerWidget{} })
	r.Register(dom.TypeMarkdown, func() Widget { return &MarkdownWidget{} })
	r.Register(dom.TypeSparkline, func() Widget { return &SparklineWidget{} })
	return r
}
