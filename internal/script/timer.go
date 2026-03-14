package script

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/dop251/goja"
)

const (
	defaultMaxTimerDuration    = 5 * time.Second
	defaultMaxConcurrentTimers = 32
)

type scriptTimer struct {
	id          int64
	ownerNodeID string
	callback    goja.Callable
	args        []goja.Value
	interval    time.Duration
	nextFire    time.Time
	repeat      bool
}

// NextTimerAt returns the next scheduled timer fire time, if any.
func (rt *Runtime) NextTimerAt() (time.Time, bool) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	return rt.nextTimerAtLocked()
}

func (rt *Runtime) nextTimerAtLocked() (time.Time, bool) {
	var next time.Time
	var ok bool
	for _, timer := range rt.timers {
		if !ok || timer.nextFire.Before(next) {
			next = timer.nextFire
			ok = true
		}
	}
	return next, ok
}

// RunDueTimers executes all timers whose due time is at or before now.
// Timer callbacks run in the same serialized VM context as hooks.
func (rt *Runtime) RunDueTimers(now time.Time) (bool, error) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	due := rt.dueTimerIDsLocked(now)
	if len(due) == 0 {
		return false, nil
	}

	rt.dirty = make(map[string]bool)

	ran := false
	var firstErr error
	for _, id := range due {
		timer, ok := rt.timers[id]
		if !ok || timer.nextFire.After(now) {
			continue
		}

		node := rt.tree.Find(timer.ownerNodeID)
		if node == nil {
			rt.clearTimerLocked(id)
			continue
		}

		if timer.repeat {
			timer.nextFire = now.Add(timer.interval)
		} else {
			rt.clearTimerLocked(id)
		}

		rt.setupContext(node, nil)

		timeout := rt.startTimeout()
		_, err := timer.callback(goja.Undefined(), timer.args...)
		rt.stopTimeout(timeout)
		if err != nil {
			rt.clearTimerLocked(id)
			if firstErr == nil {
				firstErr = wrapTimerError(timer.ownerNodeID, err)
			}
			continue
		}

		ran = true
	}

	return ran, firstErr
}

func (rt *Runtime) dueTimerIDsLocked(now time.Time) []int64 {
	type dueTimer struct {
		id       int64
		nextFire time.Time
	}

	due := make([]dueTimer, 0, len(rt.timers))
	for id, timer := range rt.timers {
		if timer.nextFire.After(now) {
			continue
		}
		due = append(due, dueTimer{id: id, nextFire: timer.nextFire})
	}

	sort.Slice(due, func(i, j int) bool {
		if due[i].nextFire.Equal(due[j].nextFire) {
			return due[i].id < due[j].id
		}
		return due[i].nextFire.Before(due[j].nextFire)
	})

	ids := make([]int64, 0, len(due))
	for _, item := range due {
		ids = append(ids, item.id)
	}
	return ids
}

func (rt *Runtime) makeSetTimerFn(ownerNodeID string, repeat bool) func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		id, err := rt.registerTimerLocked(ownerNodeID, call, repeat)
		if err != nil {
			panic(rt.vm.NewTypeError(err.Error()))
		}
		return rt.vm.ToValue(id)
	}
}

func (rt *Runtime) makeClearTimerFn() func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 || goja.IsUndefined(call.Arguments[0]) || goja.IsNull(call.Arguments[0]) {
			return goja.Undefined()
		}
		rt.clearTimerLocked(call.Arguments[0].ToInteger())
		return goja.Undefined()
	}
}

func (rt *Runtime) registerTimerLocked(ownerNodeID string, call goja.FunctionCall, repeat bool) (int64, error) {
	name := "setTimeout"
	if repeat {
		name = "setInterval"
	}
	if len(call.Arguments) == 0 {
		return 0, fmt.Errorf("%s requires a callback", name)
	}

	callback, ok := goja.AssertFunction(call.Arguments[0])
	if !ok {
		return 0, fmt.Errorf("%s requires a function callback", name)
	}
	if len(rt.timers) >= rt.maxConcurrentTimers {
		return 0, fmt.Errorf("max concurrent timers is %d", rt.maxConcurrentTimers)
	}

	delay, err := rt.parseTimerDelay(call.Argument(1))
	if err != nil {
		return 0, err
	}

	rt.nextTimerID++
	id := rt.nextTimerID
	timer := &scriptTimer{
		id:          id,
		ownerNodeID: ownerNodeID,
		callback:    callback,
		args:        append([]goja.Value(nil), call.Arguments[2:]...),
		interval:    delay,
		nextFire:    time.Now().Add(delay),
		repeat:      repeat,
	}

	rt.timers[id] = timer
	if rt.timerOwners[ownerNodeID] == nil {
		rt.timerOwners[ownerNodeID] = make(map[int64]bool)
	}
	rt.timerOwners[ownerNodeID][id] = true

	return id, nil
}

func (rt *Runtime) parseTimerDelay(value goja.Value) (time.Duration, error) {
	if value == nil || goja.IsUndefined(value) || goja.IsNull(value) {
		return 0, nil
	}

	ms := value.ToInteger()
	if ms < 0 {
		ms = 0
	}
	maxMs := int64(rt.maxTimerDuration / time.Millisecond)
	if ms > maxMs {
		return 0, fmt.Errorf("max timer duration is %s", rt.maxTimerDuration)
	}
	return time.Duration(ms) * time.Millisecond, nil
}

func (rt *Runtime) clearTimerLocked(id int64) {
	timer, ok := rt.timers[id]
	if !ok {
		return
	}
	delete(rt.timers, id)

	owned := rt.timerOwners[timer.ownerNodeID]
	delete(owned, id)
	if len(owned) == 0 {
		delete(rt.timerOwners, timer.ownerNodeID)
	}
}

func (rt *Runtime) clearNodeTimersLocked(nodeID string) {
	owned := rt.timerOwners[nodeID]
	for id := range owned {
		delete(rt.timers, id)
	}
	delete(rt.timerOwners, nodeID)
}

func wrapTimerError(nodeID string, err error) error {
	var interrupted *goja.InterruptedError
	if errors.As(err, &interrupted) {
		return &Error{
			NodeID:    nodeID,
			Hook:      "timer",
			Message:   interrupted.String(),
			IsTimeout: true,
		}
	}

	return &Error{
		NodeID:  nodeID,
		Hook:    "timer",
		Message: err.Error(),
	}
}
