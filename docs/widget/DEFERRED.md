# Widget Module: Deferred Items

Items deferred from Milestone 4 implementation. Each has a clear implementation path but was not essential for the v1 widget library.

---

## 1. Golden file tests

**What:** Visual regression tests that snapshot widget output to `testdata/golden/widget_<type>/` files and compare on subsequent runs. Regenerated with `GOLDEN_UPDATE=1`.

**Why deferred:** Lip Gloss doesn't emit ANSI escape codes in non-TTY environments (like `go test`), so golden files would capture plain text rather than styled output. Needs either a TTY simulation layer or a style-stripping normalizer to be meaningful.

**Unblocks:** Reliable visual regression detection across widget changes.

**Depends on:** Decision on how to handle TTY simulation in test harness (M5 integration may resolve this naturally via `teatest`).

---

## 2. Chroma syntax highlighting for code widget

**What:** Use [Chroma](https://github.com/alecthomas/chroma) to tokenize source code and map token types to Lip Gloss styles. The `language` prop already exists on the code widget but is currently ignored.

**Why deferred:** Adds a new dependency and requires a Chroma-to-Lip Gloss token mapping (~10 categories: keyword, string, comment, number, operator, etc.). The code widget works without it — content renders as plain text with line numbers and highlighting.

**Unblocks:** Syntax-colored code display in the terminal.

**Depends on:** Nothing — can be added independently.

---

## 3. Split mode for diff widget

**What:** Side-by-side diff rendering where old and new versions are shown in two columns. The `mode` prop and `d` toggle key already exist; split mode currently falls through to unified rendering.

**Why deferred:** Split mode is substantially harder than unified — requires synchronized scrolling, line alignment for changed blocks, and dividing available width in half minus gutter. Unified mode covers the primary use case.

**Unblocks:** More readable diffs for large changes.

**Depends on:** Nothing — can be added independently.

---

## 4. OSC 52 clipboard yank for code widget

**What:** Pressing `y` on the code widget emits an [OSC 52](https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h3-Operating-System-Commands) escape sequence to copy highlighted lines to the system clipboard.

**Why deferred:** Not all terminals support OSC 52. The implementation is straightforward (emit the escape sequence, terminals that support it will copy, others silently ignore), but requires a way to output raw escape sequences through the BubbleTea render pipeline.

**Unblocks:** Copy-to-clipboard from code blocks without leaving the TUI.

**Depends on:** M5 (BubbleTea integration) — needs a way to emit raw escape sequences outside the normal View() render path.

---

## 5. Focus cycling (Tab/Shift-Tab) in container

**What:** Container widget maintains a focus ring of focusable descendant node IDs. Tab/Shift-Tab advances the focused node. The container's `Update()` handles Tab by walking the ring and returning the new focused ID as an event.

**Why deferred:** Focus is a runtime concern — the single `focusedID` is owned by the M5 BubbleTea model and passed down via `ViewContext.Focused`. Implementing focus cycling in the container widget requires the runtime to be wired up to act on the returned focus-change event.

**Unblocks:** Keyboard-driven navigation between interactive widgets.

**Depends on:** M5 (BubbleTea integration) — needs the runtime focus management loop.

---

## 6. `append_lines` merge hook for log widget

**What:** A patch engine extension that gives the log widget append-not-replace semantics. When a `patch` operation updates a log node with an `append_lines` prop, those lines are appended to the existing `lines` array rather than replacing it, and the `append_lines` key is removed.

**Why deferred:** Requires modifying the M1 patch engine (`Tree.Patch()`) to support a `MergeHook` — a callback that customizes prop merging for specific node types. This crosses the DOM/widget boundary and needs careful design to keep the DOM layer free of widget-specific knowledge.

**Proposed design:**
```go
// MergeHook is called during patch update to customize prop merging.
// If it returns true, the hook handled the prop; the patch engine skips
// its default merge for that key.
type MergeHook func(node *dom.Node, key string, value any) bool
```

**Unblocks:** Efficient log streaming — Claude can append lines without re-sending the entire log history.

**Depends on:** M2 (MCP patch engine extension point).
