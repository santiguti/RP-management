package workorder

import "errors"

type Status string

const (
	StatusReceived     Status = "received"
	StatusDiagnosing   Status = "diagnosing"
	StatusQuoted       Status = "quoted"
	StatusApproved     Status = "approved"
	StatusRejected     Status = "rejected"
	StatusInRepair     Status = "in_repair"
	StatusWaitingParts Status = "waiting_parts"
	StatusReady        Status = "ready"
	StatusDelivered    Status = "delivered"
	StatusCancelled    Status = "cancelled"
)

type Event string

const (
	EventStartDiagnosis   Event = "start_diagnosis"
	EventQuote            Event = "quote"
	EventApprove          Event = "approve"
	EventReject           Event = "reject"
	EventStartRepair      Event = "start_repair"
	EventMarkWaitingParts Event = "mark_waiting_parts"
	EventResumeRepair     Event = "resume_repair"
	EventMarkReady        Event = "mark_ready"
	EventDeliver          Event = "deliver"
	EventCancel           Event = "cancel"
)

var (
	ErrInvalidTransition = errors.New("invalid transition")
	ErrUnknownStatus     = errors.New("unknown status")
	ErrUnknownEvent      = errors.New("unknown event")
)

var transitions = map[Status]map[Event]Status{
	StatusReceived: {
		EventStartDiagnosis: StatusDiagnosing,
		EventStartRepair:    StatusInRepair,
		EventCancel:         StatusCancelled,
	},
	StatusDiagnosing: {
		EventQuote:       StatusQuoted,
		EventStartRepair: StatusInRepair,
		EventCancel:      StatusCancelled,
	},
	StatusQuoted: {
		EventApprove: StatusApproved,
		EventReject:  StatusRejected,
		EventCancel:  StatusCancelled,
	},
	StatusApproved: {
		EventStartRepair: StatusInRepair,
		EventCancel:      StatusCancelled,
	},
	StatusRejected: {
		EventDeliver: StatusDelivered,
		EventCancel:  StatusCancelled,
	},
	StatusInRepair: {
		EventMarkWaitingParts: StatusWaitingParts,
		EventMarkReady:        StatusReady,
		EventCancel:           StatusCancelled,
	},
	StatusWaitingParts: {
		EventResumeRepair: StatusInRepair,
		EventCancel:       StatusCancelled,
	},
	StatusReady: {
		EventDeliver: StatusDelivered,
	},
	StatusDelivered: {},
	StatusCancelled: {},
}

var knownEvents = map[Event]struct{}{
	EventStartDiagnosis:   {},
	EventQuote:            {},
	EventApprove:          {},
	EventReject:           {},
	EventStartRepair:      {},
	EventMarkWaitingParts: {},
	EventResumeRepair:     {},
	EventMarkReady:        {},
	EventDeliver:          {},
	EventCancel:           {},
}

var eventOrder = []Event{
	EventStartDiagnosis,
	EventQuote,
	EventApprove,
	EventReject,
	EventStartRepair,
	EventMarkWaitingParts,
	EventResumeRepair,
	EventMarkReady,
	EventDeliver,
	EventCancel,
}

// Next returns the resulting status for the given (current, event) pair,
// or ErrInvalidTransition if the edge isn't allowed.
func Next(current Status, event Event) (Status, error) {
	events, ok := transitions[current]
	if !ok {
		return "", ErrUnknownStatus
	}
	if _, ok := knownEvents[event]; !ok {
		return "", ErrUnknownEvent
	}
	next, ok := events[event]
	if !ok {
		return "", ErrInvalidTransition
	}
	return next, nil
}

// AllowedEvents returns the events available from the current status.
func AllowedEvents(s Status) []Event {
	events, ok := transitions[s]
	if !ok || len(events) == 0 {
		return []Event{}
	}
	allowed := make([]Event, 0, len(events))
	for _, event := range eventOrder {
		if _, ok := events[event]; ok {
			allowed = append(allowed, event)
		}
	}
	return allowed
}
