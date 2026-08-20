package intent

import (
	"errors"
	"strings"
	"time"
)

// Phase is the temporal state derived from an intent and the current time.
type Phase string

const (
	PhasePending   Phase = "pending"
	PhaseActive    Phase = "active"
	PhaseEnded     Phase = "ended"
	PhaseCancelled Phase = "cancelled"
)

// Intent is the durable canonical webhook contract.
type Intent struct {
	Source    string    `json:"source,omitempty"`
	ID        string    `json:"id"`
	Subject   string    `json:"subject"`
	StartsAt  time.Time `json:"starts_at"`
	EndsAt    time.Time `json:"ends_at"`
	Cancelled bool      `json:"cancelled"`
	CreatedAt time.Time `json:"created_at,omitzero"`
	UpdatedAt time.Time `json:"updated_at,omitzero"`
}

// Validate checks the canonical intent fields independently of transport.
func (i *Intent) Validate() error {
	i.Source = strings.TrimSpace(i.Source)
	i.ID = strings.TrimSpace(i.ID)
	i.Subject = strings.TrimSpace(i.Subject)
	if i.Source == "" {
		return errors.New("source is required")
	}
	if i.ID == "" {
		return errors.New("id is required")
	}
	if i.Subject == "" {
		return errors.New("subject is required")
	}
	if i.StartsAt.IsZero() {
		return errors.New("starts_at is required")
	}
	if i.EndsAt.IsZero() {
		return errors.New("ends_at is required")
	}
	if !i.StartsAt.Before(i.EndsAt) {
		return errors.New("starts_at must precede ends_at")
	}
	return nil
}

// PhaseAt derives the intent phase at now.
func (i Intent) PhaseAt(now time.Time) Phase {
	if i.Cancelled {
		return PhaseCancelled
	}
	if now.Before(i.StartsAt) {
		return PhasePending
	}
	if now.Before(i.EndsAt) {
		return PhaseActive
	}
	return PhaseEnded
}

// NextTransitionAt returns the next phase boundary after now, if any.
func (i Intent) NextTransitionAt(now time.Time) *time.Time {
	if i.Cancelled || !now.Before(i.EndsAt) {
		return nil
	}
	transition := i.EndsAt
	if now.Before(i.StartsAt) {
		transition = i.StartsAt
	}
	return &transition
}
