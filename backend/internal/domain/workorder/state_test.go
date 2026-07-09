package workorder

import (
	"errors"
	"slices"
	"testing"
)

func TestNextAllowedTransitions(t *testing.T) {
	tests := []struct {
		name    string
		current Status
		event   Event
		want    Status
	}{
		{
			name:    "received starts diagnosis",
			current: StatusReceived,
			event:   EventStartDiagnosis,
			want:    StatusDiagnosing,
		},
		{
			name:    "received fast tracks to repair",
			current: StatusReceived,
			event:   EventStartRepair,
			want:    StatusInRepair,
		},
		{
			name:    "received cancels",
			current: StatusReceived,
			event:   EventCancel,
			want:    StatusCancelled,
		},
		{
			name:    "diagnosing quotes",
			current: StatusDiagnosing,
			event:   EventQuote,
			want:    StatusQuoted,
		},
		{
			name:    "diagnosing starts repair",
			current: StatusDiagnosing,
			event:   EventStartRepair,
			want:    StatusInRepair,
		},
		{
			name:    "diagnosing cancels",
			current: StatusDiagnosing,
			event:   EventCancel,
			want:    StatusCancelled,
		},
		{
			name:    "quoted approves",
			current: StatusQuoted,
			event:   EventApprove,
			want:    StatusApproved,
		},
		{
			name:    "quoted rejects",
			current: StatusQuoted,
			event:   EventReject,
			want:    StatusRejected,
		},
		{
			name:    "quoted cancels",
			current: StatusQuoted,
			event:   EventCancel,
			want:    StatusCancelled,
		},
		{
			name:    "approved starts repair",
			current: StatusApproved,
			event:   EventStartRepair,
			want:    StatusInRepair,
		},
		{
			name:    "approved cancels",
			current: StatusApproved,
			event:   EventCancel,
			want:    StatusCancelled,
		},
		{
			name:    "rejected delivers unrepaired device",
			current: StatusRejected,
			event:   EventDeliver,
			want:    StatusDelivered,
		},
		{
			name:    "rejected cancels",
			current: StatusRejected,
			event:   EventCancel,
			want:    StatusCancelled,
		},
		{
			name:    "in repair waits for parts",
			current: StatusInRepair,
			event:   EventMarkWaitingParts,
			want:    StatusWaitingParts,
		},
		{
			name:    "in repair marks ready",
			current: StatusInRepair,
			event:   EventMarkReady,
			want:    StatusReady,
		},
		{
			name:    "in repair cancels",
			current: StatusInRepair,
			event:   EventCancel,
			want:    StatusCancelled,
		},
		{
			name:    "waiting parts resumes repair",
			current: StatusWaitingParts,
			event:   EventResumeRepair,
			want:    StatusInRepair,
		},
		{
			name:    "waiting parts cancels",
			current: StatusWaitingParts,
			event:   EventCancel,
			want:    StatusCancelled,
		},
		{
			name:    "ready delivers",
			current: StatusReady,
			event:   EventDeliver,
			want:    StatusDelivered,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Next(tt.current, tt.event)
			if err != nil {
				t.Fatalf("Next(%q, %q) unexpected error: %v", tt.current, tt.event, err)
			}
			if got != tt.want {
				t.Fatalf("Next(%q, %q) = %q, want %q", tt.current, tt.event, got, tt.want)
			}
		})
	}
}

func TestNextInvalidTransitions(t *testing.T) {
	tests := []struct {
		current Status
		events  []Event
	}{
		{current: StatusReceived, events: []Event{EventQuote, EventDeliver}},
		{current: StatusDiagnosing, events: []Event{EventApprove, EventDeliver}},
		{current: StatusQuoted, events: []Event{EventStartDiagnosis, EventDeliver}},
		{current: StatusApproved, events: []Event{EventApprove, EventDeliver}},
		{current: StatusRejected, events: []Event{EventStartRepair, EventMarkReady}},
		{current: StatusInRepair, events: []Event{EventQuote, EventDeliver}},
		{current: StatusWaitingParts, events: []Event{EventMarkReady, EventDeliver}},
		{current: StatusReady, events: []Event{EventCancel, EventStartRepair}},
		{current: StatusDelivered, events: []Event{EventCancel, EventStartRepair}},
		{current: StatusCancelled, events: []Event{EventDeliver, EventStartRepair}},
	}

	for _, tt := range tests {
		for _, event := range tt.events {
			t.Run(string(tt.current)+" disallows "+string(event), func(t *testing.T) {
				_, err := Next(tt.current, event)
				if !errors.Is(err, ErrInvalidTransition) {
					t.Fatalf("Next(%q, %q) error = %v, want %v", tt.current, event, err, ErrInvalidTransition)
				}
			})
		}
	}
}

func TestNextUnknownInputs(t *testing.T) {
	_, err := Next("bogus", EventApprove)
	if !errors.Is(err, ErrUnknownStatus) {
		t.Fatalf("Next with unknown status error = %v, want %v", err, ErrUnknownStatus)
	}

	_, err = Next(StatusReceived, "bogus")
	if !errors.Is(err, ErrUnknownEvent) {
		t.Fatalf("Next with unknown event error = %v, want %v", err, ErrUnknownEvent)
	}
}

func TestAllowedEvents(t *testing.T) {
	tests := []struct {
		status Status
		want   []Event
	}{
		{
			status: StatusReceived,
			want:   []Event{EventStartDiagnosis, EventStartRepair, EventCancel},
		},
		{
			status: StatusDiagnosing,
			want:   []Event{EventQuote, EventStartRepair, EventCancel},
		},
		{
			status: StatusQuoted,
			want:   []Event{EventApprove, EventReject, EventCancel},
		},
		{
			status: StatusApproved,
			want:   []Event{EventStartRepair, EventCancel},
		},
		{
			status: StatusRejected,
			want:   []Event{EventDeliver, EventCancel},
		},
		{
			status: StatusInRepair,
			want:   []Event{EventMarkWaitingParts, EventMarkReady, EventCancel},
		},
		{
			status: StatusWaitingParts,
			want:   []Event{EventResumeRepair, EventCancel},
		},
		{
			status: StatusReady,
			want:   []Event{EventDeliver},
		},
		{
			status: StatusDelivered,
			want:   []Event{},
		},
		{
			status: StatusCancelled,
			want:   []Event{},
		},
		{
			status: "bogus",
			want:   []Event{},
		},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			got := AllowedEvents(tt.status)
			if !slices.Equal(got, tt.want) {
				t.Fatalf("AllowedEvents(%q) = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}
