package script

import (
	"github.com/dop251/goja"
	"github.com/joncooper/imagine-tui/internal/dom"
)

// nodeProxy implements goja.DynamicObject to bridge JS property access
// to DOM node reads/writes.
type nodeProxy struct {
	rt   *Runtime
	node *dom.Node // nil for null-ish proxy (nonexistent node)
}

// readOnlyProps that cannot be written via the proxy.
var readOnlyProps = map[string]bool{
	"id":       true,
	"type":     true,
	"children": true,
}

func (p *nodeProxy) Get(key string) goja.Value {
	if p.node == nil {
		return goja.Undefined()
	}
	switch key {
	case "id":
		return p.rt.vm.ToValue(p.node.ID)
	case "type":
		return p.rt.vm.ToValue(string(p.node.Type))
	case "value":
		return p.propValue("value")
	case "props":
		return p.rt.vm.ToValue(p.node.Props)
	case "style":
		return p.propValue("style")
	case "text":
		return p.propValue("text")
	case "visible":
		v, ok := p.node.GetProp("visible")
		if !ok {
			return p.rt.vm.ToValue(true) // default visible
		}
		return p.rt.vm.ToValue(v)
	case "children":
		return p.rt.makeChildArray(p.node)
	case "rows":
		return p.propValue("rows")
	default:
		return p.propValue(key)
	}
}

func (p *nodeProxy) Set(key string, val goja.Value) bool {
	if p.node == nil {
		return false
	}
	if readOnlyProps[key] {
		return false
	}
	p.node.SetProp(key, val.Export())
	p.rt.markDirty(p.node.ID)
	return true
}

func (p *nodeProxy) Has(key string) bool {
	if p.node == nil {
		return false
	}
	switch key {
	case "id", "type", "value", "props", "style", "text", "visible", "children", "rows":
		return true
	default:
		_, ok := p.node.GetProp(key)
		return ok
	}
}

func (p *nodeProxy) Delete(key string) bool {
	return false
}

func (p *nodeProxy) Keys() []string {
	if p.node == nil {
		return nil
	}
	keys := []string{"id", "type", "value", "props", "style", "text", "visible", "children"}
	for k := range p.node.Props {
		// Avoid duplicates with the well-known keys above.
		switch k {
		case "value", "style", "text", "visible":
			continue
		default:
			keys = append(keys, k)
		}
	}
	return keys
}

// propValue is a helper that returns a prop as a goja.Value, or undefined.
func (p *nodeProxy) propValue(key string) goja.Value {
	v, ok := p.node.GetProp(key)
	if !ok {
		return goja.Undefined()
	}
	return p.rt.vm.ToValue(v)
}

// makeChildArray returns a JS array of nodeProxy objects for the node's children.
// Must be called with rt.mu held.
func (rt *Runtime) makeChildArray(node *dom.Node) goja.Value {
	arr := make([]interface{}, len(node.Children))
	for i, child := range node.Children {
		arr[i] = rt.vm.NewDynamicObject(&nodeProxy{rt: rt, node: child})
	}
	return rt.vm.ToValue(arr)
}

// markDirty records that a node's props were modified by a script.
// Must be called with rt.mu held.
func (rt *Runtime) markDirty(nodeID string) {
	rt.dirty[nodeID] = true
}
