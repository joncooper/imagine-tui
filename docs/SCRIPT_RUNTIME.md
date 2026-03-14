# Milestone 3: Script Runtime (goja Integration)

## Context

M0 (skeleton), M1 (DOM), and M2 (MCP server) are complete and well-tested. M3 builds the goja-based script runtime in `internal/script/`, enabling Claude-authored JavaScript to execute against the DOM. This is the highest-risk milestone (goja edge cases, sandboxing) and the gate for M4 (widgets) which depends on scripts for default behaviors.

The script package currently contains only `doc.go` and a trivial goja import test. All implementation is greenfield. TDD is mandatory: write tests first, confirm they fail, then implement.

## Architecture Overview

**One goja VM per session.** The `Runtime` struct owns a single `goja.Runtime`, the DOM tree reference, the event queue, per-node state objects, and a dependency graph for computed props. All script execution is serialized by a mutex — concurrent access deferred to M5.

**$ is both an object and a function.** Use goja's `ProxyTrapConfig` wrapping a callable function: `Get`/`Set` traps delegate to a `nodeProxy` for property access (`$.value`), while the `Apply` trap handles `$('id')` lookups. Cross-node proxies returned by `$('id')` are plain `DynamicObject`s.

**Scripts mutate the DOM directly.** Write operations on `$` proxies call `node.SetProp()` and mark the node dirty. `emit('local', ops)` calls `tree.Patch()` synchronously. `emit('agent', data)` calls `events.Enqueue()`. After each script execution, dirty nodes trigger computed prop re-evaluation via the dependency graph.

## File Layout

```
internal/script/
├── runtime.go     — Runtime struct, New(), initSandbox(), execScript(), timeout
├── proxy.go       — nodeProxy (DynamicObject), setupContext() with ProxyTrapConfig
├── emit.go        — makeEmitFn(): local -> tree.Patch(), agent -> events.Enqueue()
├── state.go       — per-node state map[string]*goja.Object, getOrCreate/remove
├── hooks.go       — ExecHook(), NotifyMount/Remove/Change, public API
├── computed.go    — DepGraph (forward+reverse maps), EvalComputed(), propagateChanges(), cycle detection
├── errors.go      — ScriptError{NodeID, Hook, Message, IsTimeout}
└── *_test.go      — one test file per backlog item (sandbox, proxy, emit, state, hooks, computed)
```

## Core Struct

```go
type Runtime struct {
    mu          sync.Mutex
    vm          *goja.Runtime
    tree        *dom.Tree
    events      *dom.EventQueue
    states      map[string]*goja.Object   // nodeID -> persistent JS state
    deps        *DepGraph                  // computed prop dependency tracking
    dirty       map[string]bool            // nodes modified since last propagation
    currentEval *evalCtx                   // non-nil during computed prop eval (for dep recording)
    timeout     time.Duration              // per-script timeout (default 10ms)
    debugLog    func(string)               // optional server-side debug sink
}
```

## Public API

```go
func New(tree *dom.Tree, events *dom.EventQueue, opts ...Option) *Runtime
func (rt *Runtime) ExecHook(nodeID string, hook HookType, payload *HookPayload) (dirtyIDs []string, err error)
func (rt *Runtime) EvalComputed(nodeID, propName, expr string) (any, error)
func (rt *Runtime) EvalAllComputed() error
func (rt *Runtime) NotifyMount(nodeID string) error
func (rt *Runtime) NotifyRemove(nodeID string)
func (rt *Runtime) NotifyChange(nodeID string, newValue any) error
```

---

## Implementation Steps (in dependency order)

### Step 1: M3-1 — Sandbox Setup
**Files:** `runtime.go`, `errors.go`, `sandbox_test.go`

- [x] `Runtime` struct with `New()` constructor, `goja.Runtime` init
- [x] `initSandbox()`: delete `require`, `console`, `fetch`, `setTimeout`, `setInterval`, `process`, `globalThis`; provide `debug()` routing to server log
- [x] Timeout mechanism: `time.AfterFunc(rt.timeout, func() { rt.vm.Interrupt(...) })` + `vm.ClearInterrupt()`
- [x] `Error` type (renamed from `ScriptError` per lint) with `NodeID`, `Hook`, `Message`, `IsTimeout`
- [x] Test: forbidden API access returns error (not panic)
- [x] Test: syntax error → Error
- [x] Test: runtime error → Error
- [x] Test: infinite loop (`while(true){}`) interrupted by timeout
- [x] Test: `debug()` routes to log sink

### Step 2: M3-5 — State Object
**Files:** `state.go`, `state_test.go`

- [x] `states map[string]*goja.Object` in Runtime
- [x] `getOrCreateState(nodeID)` → returns existing or new empty `goja.Object`
- [x] `removeState(nodeID)` → deletes entry
- [x] State bound as `state` global during script execution
- [x] Test: state persists across two invocations on same node
- [x] Test: state isolated between different nodes
- [x] Test: state survives DOM prop update (patch doesn't reset state)
- [x] Test: state gone after `removeState()` call

### Step 3: M3-2 — $ API Current Node
**Files:** `proxy.go`, `proxy_test.go`

- [x] `nodeProxy` implementing goja's `DynamicObject` interface: `Get`, `Set`, `Has`, `Delete`, `Keys`
- [x] Property surface: `id` (read-only), `type` (read-only), `value`, `props`, `style`, `text`, `visible`, `children` (read-only), `rows`
- [x] `Get` reads from `node.GetProp()` / direct fields; `Set` calls `node.SetProp()` and marks dirty
- [x] Unknown keys fall through to props
- [x] `setupContext(node, payload)`: binds `$` proxy, `state`, `emit`, `event` globals
- [x] Test: read each property reflects DOM state
- [x] Test: write `$.value` updates DOM
- [x] Test: write `$.visible = false` updates DOM
- [x] Test: write to `$.id` fails (returns false)
- [x] Test: dirty set populated on write
- [x] Test: `$.children` returns child proxies

### Step 4: M3-3 — $ API Cross-Node Access
**Files:** extend `proxy.go`, `proxy_test.go`

- [x] Make `$` callable via `goja.NewProxy()` with `ProxyTrapConfig`: `Get`/`Set` → current node proxy, `Apply` → tree.Find(id) → new nodeProxy
- [x] `$('nonexistent')` returns a nodeProxy with `node: nil` (returns undefined on reads, false on writes, no crash)
- [x] Record `$('id')` calls via `recordDep()` stub (wired in M3-7)
- [x] Test: `$('existing').text` returns correct value
- [x] Test: `$('existing').text = 'new'` mutates target node
- [x] Test: `$('missing').text` returns undefined
- [x] Test: `$('missing').text = 'x'` doesn't crash
- [x] Test: chained access `$('a').value + $('b').value`

### Step 5: M3-4 — emit()
**Files:** `emit.go`, `emit_test.go`

- [x] `makeEmitFn(sourceNodeID)` returns a Go function exposed to JS
- [x] `emit('local', [...ops])`: marshal JS value → JSON → `dom.ParsePatchOps()` → `tree.Patch()`
- [x] `emit('agent', {data})`: build `dom.Event{Type: "script", Source: nodeID, Data: data}` → `events.Enqueue()`
- [x] Invalid target → JS TypeError via `panic(rt.vm.NewTypeError(...))`
- [x] Test: `emit('local', [{op:'update', id:'x', props:{text:'hi'}}])` applies patch to tree
- [x] Test: `emit('local', invalidOps)` throws
- [x] Test: `emit('agent', {action:'submit'})` enqueues event with correct source/data
- [x] Test: `emit('bad', {})` throws TypeError

### Step 6: M3-6 — Script Lifecycle Hooks
**Files:** `hooks.go`, `hooks_test.go`

- [x] Hook types: `on_mount`, `on_change`, `on_event`, `on_focus`, `on_blur`, `on_key`
- [x] `ExecHook(nodeID, hook, payload)`: find node → check `node.Scripts[hook]` exists → `execScript()` → return dirty IDs
- [x] `NotifyMount(nodeID)`: fires `on_mount` if script exists
- [x] `NotifyRemove(nodeID)`: calls `removeState()`
- [x] `NotifyChange(nodeID, newValue)`: sets prop `value` → fires `on_change`
- [x] `HookPayload` struct: `Event`, `Source`, `Key`, `Data` fields
- [x] Test: `on_mount` fires after NotifyMount
- [x] Test: `on_change` fires with correct value in payload
- [x] Test: `on_key` receives key string
- [ ] Test: `on_event` fires on parent when child emits (deferred to M5 integration)
- [x] Test: hook with timeout gets killed and returns Error
- [x] Test: hook that modifies DOM returns correct dirty set
- [x] Test: hook on nonexistent node returns error

### Step 7: M3-7 — Computed Props
**Files:** `computed.go`, `computed_test.go`

- [x] `DepGraph` with forward map (`depKey → set of nodeIDs read`) and reverse map (`nodeID → []depKey`)
- [x] `depKey{NodeID, PropName}` identifies a computed prop
- [x] `EvalComputed(nodeID, propName, expr)`: set `currentEval`, run expression, capture `$('id')` calls as deps, update DepGraph, return exported value
- [x] `PropagateChanges()`: iterate dirty nodes → find dependents via reverse map → re-evaluate → set prop → loop until no new dirty (with iteration cap)
- [x] Cycle detection: follows dirty propagation chain to detect if re-evaluating a computed prop would trigger itself
- [x] `EvalAllComputed()`: walk tree, evaluate all nodes' computed props recursively
- [x] Test: basic computed prop evaluates correctly
- [x] Test: cross-node read in computed prop
- [x] Test: dependency tracking recorded correctly
- [x] Test: changing dependency triggers re-eval via PropagateChanges
- [x] Test: dep graph updated when expression reads different nodes on re-eval
- [x] Test: `A depends on B depends on A` → cycle error
- [x] Test: error in expression → Error with node ID
- [x] Test: computed prop depending on another computed prop (transitive deps)
- [x] Test: EvalAllComputed evaluates all computed props in tree

---

## Key Design Decisions

- **One VM per session** (not per-node): `state` must persist; creating VMs is expensive
- **DynamicObject for nodeProxy + ProxyTrapConfig for $**: cleanest way to make `$` both an object and callable
- **Nil-node proxy for $('missing')**: returns undefined on reads, no crash — matches JS semantics
- **Synchronous emit('local')**: patches apply immediately within the script, no async
- **Timeout via vm.Interrupt**: goja's built-in cooperative interruption mechanism
- **Explicit NotifyMount/Remove**: caller (M5) calls these after DOM ops; avoids coupling DOM to scripts
- **Defer thread safety to M5**: internal mutex serializes script calls; multi-goroutine integration designed in M5

## Verification

After each step:
- [x] `go test ./internal/script/ -v` — all tests pass
- [x] `go test ./...` — no regressions
- [x] `golangci-lint run` — no lint issues

After all steps:
- [x] Integration test: full-stack test with DOM tree, hooks, computed props, emit, state, and cross-node access
- [x] Coverage: 88.2% overall; $ API proxy at 100%, hooks at 100%, state at 100%
