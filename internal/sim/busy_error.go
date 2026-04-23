package sim

// BusyReason describes why the engine/world rejected work as busy.
// It is a low-cardinality string so it can be safely exported in debug/metrics.
type BusyReason string

const (
	BusyReasonIntentChFull     BusyReason = "intent_ch_full"
	BusyReasonPendingQueueFull BusyReason = "pending_queue_full"
	BusyReasonAcceptTimeout    BusyReason = "accept_timeout"
)

// BusyError wraps ErrBusy with a machine-readable reason.
// It remains compatible with errors.Is(err, ErrBusy).
type BusyError struct {
	Reason BusyReason
}

func (e BusyError) Error() string {
	return "busy: " + string(e.Reason)
}

func (e BusyError) Is(target error) bool {
	return target == ErrBusy
}

