package glpi

import (
	"context"
	"sort"
	"sync"
)

// TicketEvent is one entry in the ticket timeline — followup, task, or solution
// — normalized so the UI renders them in one chronological list. (Moved from the
// api handler.)
type TicketEvent struct {
	Type      string `json:"type"` // "followup" | "task" | "solution"
	ID        int    `json:"id"`
	Content   string `json:"content"`
	Date      string `json:"date"`
	UserID    int    `json:"user_id"`
	UserName  string `json:"user_name,omitempty"`
	IsPrivate bool   `json:"is_private,omitempty"`
	State     int    `json:"state,omitempty"`  // tasks only (0=info 1=todo 2=done)
	Status    int    `json:"status,omitempty"` // solutions only (1=proposed 2=accepted 3=refused)
}

// TicketDetail is a ticket plus its merged timeline, ready to serialize. The
// JSON shape matches what GET /api/glpi/tickets/{id}/details has always returned.
type TicketDetail struct {
	Ticket      map[string]any `json:"ticket"`
	GLPIBaseURL string         `json:"glpi_base_url"`
	Requester   map[string]any `json:"requester"`
	Events      []TicketEvent  `json:"events"`
	EventCounts map[string]int `json:"event_counts"`
	Warnings    []string       `json:"warnings,omitempty"`
}

// TicketDetails fetches a ticket and its followups, tasks and solutions, resolves
// the referenced users, and merges everything into a chronological timeline.
// Extracted verbatim from the api handler (R4b): the fan-out + user resolution +
// timeline assembly is GLPI domain logic; the handler keeps only param parsing,
// auth, and the JSON response.
func (c *Client) TicketDetails(ctx context.Context, sessionToken string, ticketID int) (*TicketDetail, error) {
	t, err := c.GetTicket(ctx, sessionToken, ticketID)
	if err != nil {
		return nil, err
	}

	// Fan out the three related collections in parallel. Each failure degrades
	// gracefully to an empty slice + warning so the ticket body still renders.
	var (
		followups []Followup
		tasks     []Task
		solutions []Solution
		warnings  []string
		wg        sync.WaitGroup
		mu        sync.Mutex
	)
	addWarn := func(s string) { mu.Lock(); warnings = append(warnings, s); mu.Unlock() }

	wg.Add(3)
	go func() {
		defer wg.Done()
		f, err := c.GetTicketFollowups(ctx, sessionToken, ticketID)
		if err != nil {
			addWarn("followups: " + FriendlyError(err))
			return
		}
		followups = f
	}()
	go func() {
		defer wg.Done()
		ts, err := c.GetTicketTasks(ctx, sessionToken, ticketID)
		if err != nil {
			addWarn("tasks: " + FriendlyError(err))
			return
		}
		tasks = ts
	}()
	go func() {
		defer wg.Done()
		sols, err := c.GetTicketSolutions(ctx, sessionToken, ticketID)
		if err != nil {
			addWarn("solutions: " + FriendlyError(err))
			return
		}
		solutions = sols
	}()
	wg.Wait()

	// Resolve every referenced user id once. Cache to avoid re-fetching the same
	// user across many followups written by the same technician.
	userIDs := map[int]struct{}{}
	if t.UsersIDRequester > 0 {
		userIDs[t.UsersIDRequester] = struct{}{}
	}
	for _, f := range followups {
		if f.UsersID > 0 {
			userIDs[f.UsersID] = struct{}{}
		}
	}
	for _, tk := range tasks {
		if tk.UsersID > 0 {
			userIDs[tk.UsersID] = struct{}{}
		}
		if tk.UsersIDTech > 0 {
			userIDs[tk.UsersIDTech] = struct{}{}
		}
	}
	for _, s := range solutions {
		if s.UsersID > 0 {
			userIDs[s.UsersID] = struct{}{}
		}
	}
	users := map[int]string{}
	for id := range userIDs {
		u, err := c.GetUser(ctx, sessionToken, id)
		if err != nil || u == nil {
			continue
		}
		users[id] = u.DisplayName()
	}

	// Build the timeline in chronological order (oldest first, like a chat log).
	events := make([]TicketEvent, 0, len(followups)+len(tasks)+len(solutions))
	for _, f := range followups {
		events = append(events, TicketEvent{
			Type:      "followup",
			ID:        f.ID,
			Content:   f.Content,
			Date:      f.Date,
			UserID:    f.UsersID,
			UserName:  users[f.UsersID],
			IsPrivate: f.IsPrivate == 1,
		})
	}
	for _, tk := range tasks {
		events = append(events, TicketEvent{
			Type:     "task",
			ID:       tk.ID,
			Content:  tk.Content,
			Date:     tk.Date,
			UserID:   firstNonZero(tk.UsersIDTech, tk.UsersID),
			UserName: users[firstNonZero(tk.UsersIDTech, tk.UsersID)],
			State:    tk.State,
		})
	}
	for _, s := range solutions {
		events = append(events, TicketEvent{
			Type:     "solution",
			ID:       s.ID,
			Content:  s.Content,
			Date:     s.BestDate(),
			UserID:   s.UsersID,
			UserName: users[s.UsersID],
			Status:   s.Status,
		})
	}
	// Oldest → newest. Entries with no date are pushed to the end rather than the
	// front so a GLPI Solution missing a timestamp doesn't jump above real
	// follow-ups in the timeline.
	sort.SliceStable(events, func(i, j int) bool {
		a, b := events[i].Date, events[j].Date
		if a == "" && b == "" {
			return false
		}
		if a == "" {
			return false
		}
		if b == "" {
			return true
		}
		return a < b
	})

	return &TicketDetail{
		Ticket:      c.TicketView(t),
		GLPIBaseURL: c.WebBaseURL(),
		Requester:   map[string]any{"id": t.UsersIDRequester, "name": users[t.UsersIDRequester]},
		Events:      events,
		EventCounts: map[string]int{"followup": len(followups), "task": len(tasks), "solution": len(solutions)},
		Warnings:    warnings,
	}, nil
}

func firstNonZero(a, b int) int {
	if a != 0 {
		return a
	}
	return b
}
