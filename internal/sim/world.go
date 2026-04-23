package sim

import (
	"fmt"
	"log"
	"regexp"
	"sync"
	"time"

	"lobster-world-core/internal/events/spec"
	"lobster-world-core/internal/events/store"
	"lobster-world-core/internal/events/stream"
)

type world struct {
	worldID string

	es  store.EventStore
	hub *stream.Hub

	tickInterval time.Duration
	seed         int64
	baseTs       int64
	tsSeq        int64
	maxQueue     int
	idleTicks    int64

	mu        sync.Mutex
	tick      int64
	eventSeq  int64
	intentSeq int64
	queue     []queuedIntent
	tickStat  TickStat
	lastTickAt time.Time

	intentCh chan queuedIntent
	stopCh   chan struct{}
	stopOnce sync.Once

	state WorldState

	shocks *shockScheduler
}

type queuedIntent struct {
	ID     string
	Intent Intent
	Ack    chan error
	AcceptedEventID string
}

func newWorld(worldID string, tickInterval time.Duration, es store.EventStore, hub *stream.Hub, maxQueue int, intentChCap int) *world {
	if tickInterval <= 0 {
		tickInterval = 5 * time.Second
	}
	if maxQueue <= 0 {
		maxQueue = 1024
	}
	if intentChCap < 0 {
		intentChCap = 256
	}
	return &world{
		worldID:      worldID,
		es:           es,
		hub:          hub,
		tickInterval: tickInterval,
		seed:         0,
		baseTs:       1700000000,
		tsSeq:        0,
		maxQueue:     maxQueue,
		idleTicks:    0,
		intentCh:     make(chan queuedIntent, intentChCap),
		stopCh:       make(chan struct{}),
		state: WorldState{
			// Start from a small but non-zero baseline so spectator views feel alive.
			Food:       100,
			Population: 100,
			Order:      50,
			Trust:      50,
			Knowledge:  0,
			Conflict:   0,
		},
	}
}

func (w *world) setSeed(seed int64) {
	w.mu.Lock()
	w.seed = seed
	// Deterministic "logical time" base derived from seed.
	// Keeps ts > 0 and stable across runs, independent from wall clock.
	u := uint64(seed)
	w.baseTs = 1700000000 + int64(u%1000000)
	w.tsSeq = 0
	w.mu.Unlock()
}

func (w *world) setShockScheduler(s *shockScheduler) {
	w.mu.Lock()
	w.shocks = s
	w.mu.Unlock()
}

func (w *world) nextTsLocked() int64 {
	// Ensure strictly increasing timestamps within the world.
	w.tsSeq++
	return w.baseTs + w.tick*1000 + w.tsSeq
}

func (w *world) start() {
	go w.loop()
}

func (w *world) stop() {
	w.stopOnce.Do(func() {
		close(w.stopCh)
	})
}

func (w *world) submitIntent(in Intent) (string, <-chan error, error) {
	w.mu.Lock()
	w.intentSeq++
	id := fmt.Sprintf("int_%s_%d", sanitize(w.worldID), w.intentSeq)
	w.mu.Unlock()

	ack := make(chan error, 1)
	select {
	case w.intentCh <- queuedIntent{ID: id, Intent: in, Ack: ack}:
		// ok
	default:
		close(ack)
		return "", nil, BusyError{Reason: BusyReasonIntentChFull}
	}
	// Caller will wait on ack via Engine.
	return id, ack, nil
}

func (w *world) loop() {
	t := time.NewTicker(w.tickInterval)
	defer t.Stop()

	for {
		select {
		case <-w.stopCh:
			return
		case qi := <-w.intentCh:
			w.handleIntent(qi)
		case <-t.C:
			// Wall-clock observability only (does NOT affect simulation decisions).
			now := time.Now()
			w.mu.Lock()
			w.tickStat.TickCountTotal++
			w.tickStat.TickLastUnixMs = now.UnixMilli()
			if !w.lastTickAt.IsZero() {
				actual := now.Sub(w.lastTickAt).Milliseconds()
				expected := w.tickInterval.Milliseconds()
				if expected > 0 {
					d := actual - expected
					if d < 0 {
						d = -d
					}
					w.tickStat.TickJitterMsTotal += d
					w.tickStat.TickJitterCount++
					if actual >= 2*expected {
						w.tickStat.TickOverrunTotal++
					}
				}
			}
			w.lastTickAt = now
			w.mu.Unlock()
			w.step()
		}
	}
}

func (w *world) handleIntent(qi queuedIntent) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Backpressure: refuse when too many pending intents are queued.
	if w.maxQueue > 0 && len(w.queue) >= w.maxQueue {
		if qi.Ack != nil {
			select {
			case qi.Ack <- BusyError{Reason: BusyReasonPendingQueueFull}:
			default:
			}
			close(qi.Ack)
		}
		return
	}

	// Emit intent_accepted immediately at current tick (tick=0 at world start is allowed).
	ev := w.newEventLocked("intent_accepted", []string{qi.ID}, fmt.Sprintf("意图接受：%s", qi.Intent.Goal))
	ev.Trace = []spec.TraceLink{
		{CauseEventID: "", Note: "目标：" + qi.Intent.Goal},
	}
	err := w.appendAndPublish(ev)
	if err == nil {
		// Enqueue for future execution only if the acceptance event was durably written.
		qi.AcceptedEventID = ev.EventID
		w.queue = append(w.queue, qi)
	}
	if qi.Ack != nil {
		select {
		case qi.Ack <- err:
		default:
		}
		close(qi.Ack)
	}
}

func (w *world) step() {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.tick++

	// Shock scheduler runs at tick boundaries.
	shockEmitted := false
	if w.shocks != nil {
		evs := w.shocks.step(w.worldID, w.tick)
		for _, ev := range evs {
			ev.Ts = w.nextTsLocked()
			// Shock deltas directly impact world state (v0).
			w.state.ApplyDelta(ev.Delta)
			if err := w.appendAndPublish(ev); err != nil {
				log.Printf("sim: failed to persist shock event world=%s type=%s err=%v", w.worldID, ev.Type, err)
			}
			shockEmitted = true
		}
	}
	if shockEmitted {
		w.idleTicks = 0
	}

	if len(w.queue) == 0 {
		// Natural evolution runs when the world is idle for a while, and no shock event
		// happened at this tick. This avoids perturbing early deterministic timelines
		// (tests and replay), but still keeps the world alive when idle.
		if !shockEmitted {
			w.idleTicks++
			// Evolve based on elapsed wall-time via tickInterval, but without using time.Now
			// so the timeline remains deterministic for a given tick stream.
			//
			// In tests we often use very small tick intervals (e.g. 10ms). If we evolved every
			// few ticks, replay/export snapshots would change between back-to-back requests,
			// making determinism assertions flaky. Using a time-based cadence keeps the world
			// "alive" in production while remaining stable in fast-tick tests.
			const evolveEvery = 30 * time.Second
			evolveEveryIdleTicks := int64(evolveEvery / w.tickInterval)
			if evolveEveryIdleTicks < 5 {
				evolveEveryIdleTicks = 5
			}
			if w.idleTicks >= evolveEveryIdleTicks {
				if typ, narr, delta, ok := w.evolveLocked(); ok {
					pre := w.state
					ev := w.newEventLocked(typ, []string{"system"}, narr)
					ev.Tick = w.tick
					ev.Delta = delta
					ev.Trace = []spec.TraceLink{
						{CauseEventID: "", Note: fmt.Sprintf("演化前状态：食物=%d 人口=%d 秩序=%d 信任=%d 冲突=%d", pre.Food, pre.Population, pre.Order, pre.Trust, pre.Conflict)},
						{CauseEventID: "", Note: "演化原因：" + narr},
					}
					w.state.ApplyDelta(ev.Delta)
					if err := w.appendAndPublish(ev); err != nil {
						log.Printf("sim: failed to persist evolution event world=%s err=%v", w.worldID, err)
					}
				}
				w.idleTicks = 0
			}
		}
		return
	}

	// We will execute an intent this tick; reset idle counter.
	w.idleTicks = 0

	// Execute at most one intent per tick (v0 throttle; easy to reason about).
	qi := w.queue[0]
	w.queue = w.queue[1:]

	started := w.newEventLocked("action_started", []string{qi.ID}, "行动开始：执行意图")
	started.Tick = w.tick
	if qi.AcceptedEventID != "" {
		started.Trace = []spec.TraceLink{{CauseEventID: qi.AcceptedEventID, Note: "由意图进入执行阶段"}}
	}
	if err := w.appendAndPublish(started); err != nil {
		log.Printf("sim: failed to persist action_started world=%s err=%v", w.worldID, err)
		return
	}

	done := w.newEventLocked("action_completed", []string{qi.ID}, "行动完成：意图执行完毕")
	done.Tick = w.tick
	pre := w.state
	// v0 deterministic intent effects (placeholder rules engine).
	done.Delta = intentDelta(qi.Intent.Goal)
	done.Trace = append(done.Trace, spec.TraceLink{CauseEventID: started.EventID, Note: "执行完成"})
	if qi.AcceptedEventID != "" {
		done.Trace = append(done.Trace, spec.TraceLink{CauseEventID: qi.AcceptedEventID, Note: "回溯意图来源"})
	}
	done.Trace = append(done.Trace, spec.TraceLink{
		CauseEventID: "",
		Note:         fmt.Sprintf("执行前状态：食物=%d 人口=%d 秩序=%d 信任=%d 冲突=%d", pre.Food, pre.Population, pre.Order, pre.Trust, pre.Conflict),
	})
	done.Trace = append(done.Trace, spec.TraceLink{
		CauseEventID: "",
		Note:         "规则解释：" + explainIntentRule(qi.Intent.Goal),
	})
	w.state.ApplyDelta(done.Delta)
	if err := w.appendAndPublish(done); err != nil {
		log.Printf("sim: failed to persist action_completed world=%s err=%v", w.worldID, err)
		return
	}

	// v0.3-M1: optional story layer. After an intent completes, emit at most one
	// additional world-scope event (diplomacy/trade) to enrich spectator/replay.
	if extra, ok := buildStoryWorldEvent(w.worldID, w.tick, w.seed, qi.ID, qi.Intent.Goal, qi.AcceptedEventID, done.EventID); ok {
		ev := w.newEventLocked(extra.Type, extra.Actors, extra.Narrative)
		ev.Delta = extra.Delta
		ev.Trace = extra.Trace
		w.state.ApplyDelta(ev.Delta)
		if err := w.appendAndPublish(ev); err != nil {
			log.Printf("sim: failed to persist story event world=%s type=%s err=%v", w.worldID, ev.Type, err)
		}
	}
}

func (w *world) newEventLocked(typ string, actors []string, narrative string) spec.Event {
	w.eventSeq++
	return spec.Event{
		SchemaVersion: 1,
		EventID:       fmt.Sprintf("evt_%s_%d_%d", sanitize(w.worldID), w.tick, w.eventSeq),
		Ts:            w.nextTsLocked(),
		WorldID:       w.worldID,
		Scope:         "world",
		Type:          typ,
		Actors:        actors,
		Narrative:     narrative,
		Tick:          w.tick,
	}
}

func (w *world) appendAndPublish(ev spec.Event) error {
	if w.es != nil {
		if err := w.es.Append(ev); err != nil {
			return err
		}
	}
	if w.hub != nil {
		w.hub.Publish(ev)
	}
	return nil
}

type Status struct {
	WorldID string
	Tick    int64
	State   WorldState
}

func (w *world) status() Status {
	w.mu.Lock()
	defer w.mu.Unlock()

	return Status{
		WorldID: w.worldID,
		Tick:    w.tick,
		State:   w.state,
	}
}

func (w *world) queueStats() QueueStat {
	w.mu.Lock()
	defer w.mu.Unlock()
	return QueueStat{
		IntentChLen:     len(w.intentCh),
		IntentChCap:     cap(w.intentCh),
		PendingQueueLen: len(w.queue),
		PendingQueueMax: w.maxQueue,
		Tick:            w.tick,
	}
}

func (w *world) tickStats() TickStat {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.tickStat
}

var sanitizeRe = regexp.MustCompile(`[^a-zA-Z0-9]+`)

func sanitize(s string) string {
	if s == "" {
		return "world"
	}
	return sanitizeRe.ReplaceAllString(s, "_")
}
