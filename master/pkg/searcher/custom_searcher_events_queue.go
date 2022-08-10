package searcher

import (
	"fmt"

	"github.com/determined-ai/determined/proto/pkg/experimentv1"
)

// SearcherEventQueue stores the list of custom searcher events and the event
// that was event that was processed last by client and acknowledged by master.
type SearcherEventQueue struct {
	events     []*experimentv1.SearcherEvent
	EventCount int32 // stores the number of events in the queue.
	// Will help with uniquely identifying an event.
}

func newSearcherEventQueue() *SearcherEventQueue {
	events := make([]*experimentv1.SearcherEvent, 0)
	return &SearcherEventQueue{events: events, EventCount: 0}
}

// Enqueue an event.
func (q *SearcherEventQueue) Enqueue(event *experimentv1.SearcherEvent) {
	q.EventCount++
	event.Id = q.EventCount
	q.events = append(q.events, event)
}

// GetEvents returns all the events.
func (q *SearcherEventQueue) GetEvents() []*experimentv1.SearcherEvent {
	return q.events
}

// SetEvents sets the events.
func (q *SearcherEventQueue) SetEvents(events []*experimentv1.SearcherEvent) {
	q.events = events
}

// RemoveUpTo the given event Id.
func (q *SearcherEventQueue) RemoveUpTo(eventID int) error {
	for i, v := range q.events {
		if v.Id == int32(eventID) {
			q.events = q.events[i+1:]
			return nil
		}
	}
	return fmt.Errorf("event %d not found", eventID)
}
