package sim

import "errors"

// ErrBusy indicates the world/engine is overloaded and cannot accept more work right now.
var ErrBusy = errors.New("busy")

