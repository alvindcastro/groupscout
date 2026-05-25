package storage

import (
	"fmt"
	"strings"
	"time"
)

type leadActionUpdate struct {
	Status       string
	Owner        string
	SnoozedUntil *time.Time
	Flagged      bool
	Notes        string
	Changed      []string
}

func buildLeadActionUpdate(lead Lead, action LeadAction, now time.Time) (leadActionUpdate, error) {
	normalized := normalizeLeadAction(action.Action)
	if normalized == "" {
		return leadActionUpdate{}, fmt.Errorf("%w: unsupported action %q", ErrInvalidLeadTransition, action.Action)
	}
	if isTerminalLeadStatus(lead.Status) && normalized != "reopen" {
		return leadActionUpdate{}, fmt.Errorf("%w: cannot %s from %s", ErrInvalidLeadTransition, normalized, lead.Status)
	}

	update := leadActionUpdate{
		Status:       lead.Status,
		Owner:        lead.Owner,
		SnoozedUntil: lead.SnoozedUntil,
		Flagged:      lead.Flagged,
		Notes:        lead.Notes,
	}

	if err := applyLeadActionTransition(&update, normalized, action, now); err != nil {
		return leadActionUpdate{}, err
	}
	if action.Notes != nil {
		update.Notes = *action.Notes
		update.Changed = append(update.Changed, "notes")
	}

	return update, nil
}

func applyLeadActionTransition(update *leadActionUpdate, normalized string, action LeadAction, now time.Time) error {
	switch normalized {
	case "claim":
		if strings.TrimSpace(action.Owner) == "" {
			return fmt.Errorf("%w: claim requires owner", ErrInvalidLeadTransition)
		}
		update.Status = "claimed"
		update.Owner = strings.TrimSpace(action.Owner)
		update.Changed = append(update.Changed, "status", "owner")
	case "dismiss":
		update.Status = "dismissed"
		update.Changed = append(update.Changed, "status")
	case "snooze":
		if action.SnoozedUntil == nil || !action.SnoozedUntil.After(now) {
			return fmt.Errorf("%w: snooze requires future snoozed_until", ErrInvalidLeadTransition)
		}
		update.Status = "snoozed"
		update.SnoozedUntil = action.SnoozedUntil
		update.Changed = append(update.Changed, "status", "snoozed_until")
	case "flag":
		update.Status = "flagged"
		update.Flagged = true
		update.Changed = append(update.Changed, "status", "flagged")
	case "contacted":
		update.Status = "contacted"
		update.Changed = append(update.Changed, "status")
	case "won":
		update.Status = "won"
		update.Changed = append(update.Changed, "status")
	case "lost":
		update.Status = "lost"
		update.Changed = append(update.Changed, "status")
	case "no_response":
		update.Status = "no_response"
		update.Changed = append(update.Changed, "status")
	case "reopen":
		update.Status = "new"
		update.Flagged = false
		update.SnoozedUntil = nil
		update.Changed = append(update.Changed, "status", "flagged", "snoozed_until")
	}
	return nil
}

func normalizeLeadAction(action string) string {
	switch strings.TrimSpace(strings.ToLower(action)) {
	case "claim":
		return "claim"
	case "dismiss":
		return "dismiss"
	case "snooze":
		return "snooze"
	case "flag":
		return "flag"
	case "contacted":
		return "contacted"
	case "won":
		return "won"
	case "lost":
		return "lost"
	case "no-response", "no_response":
		return "no_response"
	case "reopen":
		return "reopen"
	default:
		return ""
	}
}

func isTerminalLeadStatus(status string) bool {
	switch status {
	case "won", "lost", "dismissed":
		return true
	default:
		return false
	}
}
