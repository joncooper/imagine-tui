# MCP Feedback Session — 2026-03-13

Feedback from Claude (Opus 4.6) after building an interactive "Space Launch Mission Control" demo using the imagine-tui MCP server. The demo used: container, text, table, log, and button widgets with live updates via `patch` and `set_items`.

---

## 1. The Log Widget Was the Biggest Pain Point

Attempted `append_items` on the log widget and got:

```
append_items: node "mission-log": missing item_template prop
```

This forced **resending the entire `lines` array via `patch` on every update**. For the countdown sequence, that meant shipping all previous log lines plus the new one each time — massively token-wasteful. By the end, 15+ lines were being sent just to add one.

**Suggestion:** The log widget should natively support `append_items` (or a dedicated `append_lines` tool). It's an append-only widget by nature. This would be the single biggest improvement for token efficiency.

---

## 2. Scripting Was NOT Exposed via MCP

The scripting system is **not discoverable through the MCP tools**:

- `describe_widgets` doesn't mention `scripts`, `computed`, `on_change`, `on_mount`, or any hook props on any widget
- There's no `describe_scripting` or similar discovery tool
- A fresh Claude session with no codebase access would have **zero awareness** that scripting exists

This matters because:

- **Computed props** could have replaced several `patch` calls (e.g., the header status text could react to a state change instead of being manually updated)
- **`on_change` / `on_mount` hooks** could have driven the systems-check cascade locally without round-tripping to Claude
- **`emit('local', patch)`** could have handled the countdown animation entirely client-side

**Suggestion:** Either add scripting props to the `describe_widgets` output, or add a `describe_scripting` tool that explains the `$` API, hooks, computed props, emit, and state.

---

## 3. No Way to Do Timed Sequences Without Round-Trips

The countdown required ~10 sequential `patch` calls, each a full MCP round-trip. There was no timer primitive available. The spec mentions `on_tick` as a hook — if that exists, exposing it via scripting would allow:

```json
{
  "on_tick": "if (state.count > 0) { state.count--; $('countdown').text = 'T-' + state.count; }"
}
```

That would turn 10 MCP calls into 1.

---

## 4. Smaller Feedback

| Issue | Detail |
|-------|--------|
| **`set_items` vs `patch` confusion** | For the table, `set_items` worked great. For the log, it didn't. The mental model of "which update mechanism works on which widget" wasn't clear from `describe_widgets`. |
| **No partial log update** | Even a `patch` op like `{"op": "append_prop", "id": "mission-log", "prop": "lines", "values": [...]}` would help. |
| **`row_style` is great** | The conditional row styling on the table was zero effort, big visual payoff. |
| **Widget catalog was helpful** | `describe_widgets` returning everything in one call was the right move. Planning the whole layout from a single response worked well. |
| **`dom_summary` in events is clever** | Returning the tree shape with each event gives just enough context without flooding tokens. |

---

## Priority List

1. **Log `append_items` support** — biggest token waste by far
2. **Expose scripting in `describe_widgets` / new discovery tool** — invisible to MCP clients
3. **Timer/tick primitive** — enables animations without round-trips
4. **Partial prop append** — for any array-valued prop, not just items

---

## What Worked Well

The widget set, `set_items` pattern, conditional `row_style`, and event model all felt natural. The `dom_summary` in event responses is a smart design. The main gap is that the powerful scripting layer is invisible to MCP clients.
