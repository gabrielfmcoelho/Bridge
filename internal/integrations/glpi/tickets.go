package glpi

import (
	"context"
	"fmt"
	"strings"
)

// TicketScope describes a server-side ticket search window. The field numbers
// are GLPI search-engine internals (12=status, 80=entity, 7=category; status
// value 6 = closed) — kept here so the api handlers don't carry protocol detail.
type TicketScope struct {
	IncludeClosed bool
	EntityID      int // GLPI field 80; 0 = omit this criterion
	CategoryID    int // GLPI field 7; 0 = omit this criterion
	RangeStart    int
	RangeEnd      int
}

// ListScopedTickets builds the GLPI search criteria for the given scope and runs
// the search. Extracted from the api ticket handlers (R4b) so the criteria
// assembly lives with the client.
func (c *Client) ListScopedTickets(ctx context.Context, sessionToken string, s TicketScope) ([]Ticket, error) {
	var criteria []map[string]string
	if !s.IncludeClosed {
		criteria = append(criteria, map[string]string{"field": "12", "searchtype": "notequals", "value": "6", "link": "AND"})
	}
	if s.EntityID > 0 {
		criteria = append(criteria, map[string]string{"field": "80", "searchtype": "equals", "value": fmt.Sprintf("%d", s.EntityID), "link": "AND"})
	}
	if s.CategoryID > 0 {
		criteria = append(criteria, map[string]string{"field": "7", "searchtype": "equals", "value": fmt.Sprintf("%d", s.CategoryID), "link": "AND"})
	}
	return c.SearchTickets(ctx, sessionToken, criteria, s.RangeStart, s.RangeEnd)
}

// TicketView is the display projection of a Ticket (ids, status labels, deep
// link). Moved from the api handler (enrichTicket) so GLPI presentation lives
// with the client.
func (c *Client) TicketView(t *Ticket) map[string]any {
	return map[string]any{
		"id":           t.ID,
		"name":         t.Name,
		"content":      t.Content,
		"status":       t.Status,
		"status_label": StatusLabel(t.Status),
		"status_slug":  StatusSlug(t.Status),
		"priority":     t.Priority,
		"entities_id":  t.EntitiesID,
		"date":         t.Date,
		"date_mod":     t.DateMod,
		"url":          fmt.Sprintf("%s/front/ticket.form.php?id=%d", c.WebBaseURL(), t.ID),
	}
}

// FriendlyError maps a GLPI client/transport error to a user-facing message.
// Moved from the api handler (mapGlpiError) — the HTTP-status→cause mapping is
// GLPI-specific knowledge.
func FriendlyError(err error) string {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "401"):
		return "Authentication failed — check App-Token and user token"
	case strings.Contains(msg, "403"):
		// 403 means the session is valid but the active profile lacks read
		// rights on this GLPI itemtype. Point users at the real cause.
		return "Permission denied — this GLPI profile can't read this resource"
	case strings.Contains(msg, "404"):
		return "Endpoint not found — check Base URL (should end in /apirest.php)"
	case strings.Contains(msg, "no such host"), strings.Contains(msg, "connection refused"):
		return "Host unreachable — check Base URL"
	case strings.Contains(msg, "context deadline exceeded"):
		return "Timed out — GLPI took too long"
	}
	return msg
}
