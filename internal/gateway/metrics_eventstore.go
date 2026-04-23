package gateway

import (
	"lobster-world-core/internal/events/spec"
	"lobster-world-core/internal/events/store"
)

// metricsEventStore wraps an EventStore and increments Metrics on Append.
// It forwards all read paths unchanged.
type metricsEventStore struct {
	inner store.EventStore
	mt    *Metrics
}

func wrapEventStoreWithMetrics(es store.EventStore, mt *Metrics) store.EventStore {
	if es == nil || mt == nil {
		return es
	}
	if _, ok := es.(*metricsEventStore); ok {
		return es
	}
	return &metricsEventStore{inner: es, mt: mt}
}

func (s *metricsEventStore) Append(e spec.Event) error {
	s.mt.IncEventStoreAppend()
	err := s.inner.Append(e)
	if err != nil {
		s.mt.IncEventStoreAppendError()
	}
	return err
}

func (s *metricsEventStore) Query(q store.Query) ([]spec.Event, error) {
	return s.inner.Query(q)
}

func (s *metricsEventStore) GetByID(worldID, eventID string) (spec.Event, bool, error) {
	return s.inner.GetByID(worldID, eventID)
}

// Forward neighbor lookup if supported by inner store (keeps highlight fast).
func (s *metricsEventStore) GetNeighbors(worldID, eventID string, radius int) (prev, next spec.Event, okPrev, okNext bool, err error) {
	type ns interface {
		GetNeighbors(worldID, eventID string, radius int) (prev, next spec.Event, okPrev, okNext bool, err error)
	}
	if inner, ok := s.inner.(ns); ok {
		return inner.GetNeighbors(worldID, eventID, radius)
	}
	return spec.Event{}, spec.Event{}, false, false, nil
}

