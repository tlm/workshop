package hookstate

import "github.com/canonical/workshop/internal/overlord/state"

const FakeHook = fakeHook

func NewWithoutBackend(st *state.State) *HookManager {
	return &HookManager{state: st, contexts: make(map[string]*Context)}
}
