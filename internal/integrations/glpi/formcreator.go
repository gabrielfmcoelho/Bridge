package glpi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// flexInt decodes a JSON int, a JSON string holding an int, or null/empty as
// zero. GLPI's REST stringifies several numeric fields on plugin itemtypes —
// Formcreator in particular emits "entities_id":"0" on some versions — so
// straight `int` decoding breaks. The underlying type is `int`, so callers
// compare against literals (`f.IsActive == 1`) as if it were a normal int.
type flexInt int

func (f *flexInt) UnmarshalJSON(b []byte) error {
	if len(b) == 0 || string(b) == "null" {
		return nil
	}
	if b[0] != '"' {
		// Raw number — int or float.
		var n int
		if err := json.Unmarshal(b, &n); err == nil {
			*f = flexInt(n)
			return nil
		}
		var fl float64
		if err := json.Unmarshal(b, &fl); err == nil {
			*f = flexInt(int(fl))
			return nil
		}
		return fmt.Errorf("flexInt: cannot parse %s", string(b))
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return fmt.Errorf("flexInt: %q is not an int", s)
	}
	*f = flexInt(n)
	return nil
}

// MarshalJSON keeps flexInt as a plain number on the way out so the frontend
// never sees the string form even when GLPI returned one.
func (f flexInt) MarshalJSON() ([]byte, error) { return []byte(strconv.Itoa(int(f))), nil }

// flexString accepts either a quoted string or a bare JSON number. GLPI
// stringifies some fields and keeps others numeric depending on version,
// and our frontend only cares about the string form.
type flexString string

func (f *flexString) UnmarshalJSON(b []byte) error {
	if len(b) == 0 || string(b) == "null" {
		return nil
	}
	if b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*f = flexString(s)
		return nil
	}
	// Anything else — number, bool — preserve the raw JSON lexeme.
	*f = flexString(string(b))
	return nil
}

// conditionOp is Formcreator's show_condition. In recent versions the REST
// returns the integer constant (1..9) instead of the old string enum. We
// translate during decode so frontend evaluators stay on the eq/neq/…
// vocabulary they already handle.
//
// Source: PluginFormcreatorCondition::SHOW_CONDITION_* constants in the
// Formcreator plugin repo.
type conditionOp string

func (c *conditionOp) UnmarshalJSON(b []byte) error {
	if len(b) == 0 || string(b) == "null" {
		return nil
	}
	if b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*c = conditionOp(s)
		return nil
	}
	var n int
	if err := json.Unmarshal(b, &n); err != nil {
		return err
	}
	switch n {
	case 1:
		*c = "eq"
	case 2:
		*c = "neq"
	case 3:
		*c = "lt"
	case 4:
		*c = "gt"
	case 5:
		*c = "le"
	case 6:
		*c = "ge"
	case 7:
		*c = "visible"
	case 8:
		*c = "invisible"
	case 9:
		*c = "regex"
	default:
		*c = conditionOp(strconv.Itoa(n))
	}
	return nil
}

// conditionLogic is show_logic. Same story — new Formcreator emits 1/2 for
// AND/OR instead of the string form.
type conditionLogic string

func (l *conditionLogic) UnmarshalJSON(b []byte) error {
	if len(b) == 0 || string(b) == "null" {
		return nil
	}
	if b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*l = conditionLogic(s)
		return nil
	}
	var n int
	if err := json.Unmarshal(b, &n); err != nil {
		return err
	}
	switch n {
	case 1:
		*l = "AND"
	case 2:
		*l = "OR"
	default:
		*l = conditionLogic(strconv.Itoa(n))
	}
	return nil
}

// ─── Formcreator types ──────────────────────────────────────────────────────

// FormcreatorForm mirrors PluginFormcreatorForm — the top-level form resource.
// Only the fields sshcm actually renders are kept; extra GLPI fields are
// ignored at decode time. Numeric fields use flexInt so GLPI's habit of
// emitting "1" instead of 1 (on plugin itemtypes) doesn't break decoding.
type FormcreatorForm struct {
	ID              flexInt `json:"id"`
	Name            string  `json:"name"`
	Description     string  `json:"description"`
	Content         string  `json:"content"`
	IsActive        flexInt `json:"is_active"`
	Language        string  `json:"language"`
	EntitiesID      flexInt `json:"entities_id"`
	CategoriesID    flexInt `json:"plugin_formcreator_categories_id"`
	AccessRights    flexInt `json:"access_rights"`
	Icon            string  `json:"icon"`
	IconColor       string  `json:"icon_color"`
	BackgroundColor string  `json:"background_color"`
}

// FormcreatorSection groups questions inside a form.
type FormcreatorSection struct {
	ID      flexInt `json:"id"`
	Name    string  `json:"name"`
	Order   flexInt `json:"order"`
	FormsID flexInt `json:"plugin_formcreator_forms_id"`
}

// FormcreatorQuestion is a single input rendered by FormcreatorQuestion.tsx.
// `Values` holds select options (JSON string inside the GLPI response),
// `DefaultValues` the initial value, `Range` optional numeric min/max.
type FormcreatorQuestion struct {
	ID            flexInt `json:"id"`
	Name          string  `json:"name"`
	FieldType     string  `json:"fieldtype"`
	Required      flexInt `json:"required"`
	Description   string  `json:"description"`
	DefaultValues string  `json:"default_values"`
	Values        string  `json:"values"`
	Order         flexInt `json:"order"`
	Row           flexInt `json:"row"`
	Col           flexInt `json:"col"`
	Width         flexInt `json:"width"`
	SectionsID    flexInt `json:"plugin_formcreator_sections_id"`
	RegexPattern  string  `json:"regex"`
}

// FormcreatorCondition carries a show/hide rule for either a question or a
// section. ItemType is "PluginFormcreatorQuestion" or "PluginFormcreatorSection".
// show_condition / show_logic arrive as integers in recent Formcreator
// releases — see conditionOp / conditionLogic for the translation map.
type FormcreatorCondition struct {
	ID            flexInt        `json:"id"`
	ItemType      string         `json:"itemtype"`
	ItemsID       flexInt        `json:"items_id"`
	QuestionsID   flexInt        `json:"plugin_formcreator_questions_id"`
	ShowLogic     conditionLogic `json:"show_logic"`     // AND | OR
	ShowCondition conditionOp    `json:"show_condition"` // eq | neq | lt | le | gt | ge | regex | visible | invisible
	ShowValue     flexString     `json:"show_value"`
	Order         flexInt        `json:"order"`
}

// FormcreatorFormBundle is the shape sshcm's handler returns after fanning out
// to the four underlying itemtypes.
type FormcreatorFormBundle struct {
	Form       *FormcreatorForm         `json:"form"`
	Sections   []FormcreatorSection     `json:"sections"`
	Questions  []FormcreatorQuestion    `json:"questions"`
	Conditions []FormcreatorCondition   `json:"conditions"`
}

// FormcreatorSubmitResult is what we hand back to the frontend after a
// successful PluginFormcreatorFormAnswer creation. created_tickets is
// populated when Formcreator's dispatch built tickets — sshcm doesn't
// currently enumerate changes/problems in the success pane, so they're
// only surfaced as counts.
type FormcreatorSubmitResult struct {
	FormAnswerID     int                      `json:"form_answer_id"`
	Status           int                      `json:"status"`
	URL              string                   `json:"url,omitempty"`
	CreatedTickets   []map[string]any         `json:"created_tickets,omitempty"`
	CreatedCounts    map[string]int           `json:"created_counts,omitempty"`
}

// ─── Client methods ─────────────────────────────────────────────────────────

// ListFormcreatorForms returns forms the service session can see. When
// onlyActive is true it filters to is_active=1 via a search criterion. Pass
// rangeStart=0, rangeEnd=49 for the default first page.
func (c *Client) ListFormcreatorForms(ctx context.Context, sessionToken string, onlyActive bool, rangeStart, rangeEnd int) ([]FormcreatorForm, error) {
	// The plain /PluginFormcreatorForm endpoint returns everything visible;
	// filtering is done via /search/PluginFormcreatorForm when we need it.
	if !onlyActive {
		q := url.Values{}
		q.Set("range", fmt.Sprintf("%d-%d", rangeStart, rangeEnd))
		// Explicitly keep expand_dropdowns=false: with it enabled, GLPI rewrites
		// every FK (entities_id, plugin_formcreator_categories_id, …) to its
		// human-readable name, which then fails to decode into our int fields.
		q.Set("expand_dropdowns", "false")
		var out []FormcreatorForm
		if err := c.do(ctx, "GET", "/PluginFormcreatorForm", sessionToken, q, nil, &out); err != nil {
			return nil, err
		}
		return out, nil
	}
	// Search endpoint returns a different shape ({totalcount, data:[{<fieldID>:val}]}).
	// For filtering active=1, field id 8 is `is_active` on PluginFormcreatorForm in
	// recent Formcreator builds; we ask the server to forcedisplay the columns we
	// map back to FormcreatorForm.
	q := url.Values{}
	q.Set("criteria[0][field]", "8")
	q.Set("criteria[0][searchtype]", "equals")
	q.Set("criteria[0][value]", "1")
	q.Set("forcedisplay[0]", "2")  // id
	q.Set("forcedisplay[1]", "1")  // name
	q.Set("forcedisplay[2]", "3")  // description
	q.Set("range", fmt.Sprintf("%d-%d", rangeStart, rangeEnd))
	var raw struct {
		Totalcount int              `json:"totalcount"`
		Data       []map[string]any `json:"data"`
	}
	if err := c.do(ctx, "GET", "/search/PluginFormcreatorForm", sessionToken, q, nil, &raw); err != nil {
		return nil, err
	}
	out := make([]FormcreatorForm, 0, len(raw.Data))
	for _, row := range raw.Data {
		f := FormcreatorForm{IsActive: 1}
		if v, ok := row["2"]; ok {
			f.ID = flexInt(toInt(v))
		}
		if v, ok := row["1"]; ok {
			f.Name = toString(v)
		}
		if v, ok := row["3"]; ok {
			f.Description = toString(v)
		}
		out = append(out, f)
	}
	return out, nil
}

// GetFormcreatorForm fetches a single form by id. Use this when you need the
// full metadata (content HTML, icon, colors) — the list endpoint already
// covers most rendering needs.
func (c *Client) GetFormcreatorForm(ctx context.Context, sessionToken string, id int) (*FormcreatorForm, error) {
	var f FormcreatorForm
	path := fmt.Sprintf("/PluginFormcreatorForm/%d", id)
	if err := c.do(ctx, "GET", path, sessionToken, nil, nil, &f); err != nil {
		return nil, err
	}
	return &f, nil
}

// fetchAllPluginRows pulls every row of a Formcreator itemtype in 200-row
// pages, up to `maxTotal`. We do the filtering in Go because GLPI's
// server-side filters (searchText / search/criteria) depend on field ids that
// aren't stable across Formcreator releases — one release exposed
// plugin_formcreator_forms_id as "23", another as "16". A two-page fetch is
// cheap next to that fragility.
func (c *Client) fetchAllPluginRows(ctx context.Context, sessionToken, itemtype string, maxTotal int, into any) error {
	// `into` is a pointer to a slice — we accumulate into a raw slice of maps
	// first, then json-round-trip into the typed slice to reuse the struct's
	// tags (including the flexInt decoder). Simpler than reflection gymnastics.
	const page = 200
	accum := make([]map[string]any, 0, page)
	for start := 0; start < maxTotal; start += page {
		end := start + page - 1
		if end >= maxTotal {
			end = maxTotal - 1
		}
		q := url.Values{}
		q.Set("range", fmt.Sprintf("%d-%d", start, end))
		q.Set("expand_dropdowns", "false")
		var chunk []map[string]any
		if err := c.do(ctx, "GET", "/"+itemtype, sessionToken, q, nil, &chunk); err != nil {
			return err
		}
		accum = append(accum, chunk...)
		if len(chunk) < page {
			break // short page = last page
		}
	}
	raw, err := json.Marshal(accum)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, into)
}

// ListFormcreatorSectionsByForm returns sections ordered by `order` ASC.
func (c *Client) ListFormcreatorSectionsByForm(ctx context.Context, sessionToken string, formID int) ([]FormcreatorSection, error) {
	var all []FormcreatorSection
	if err := c.fetchAllPluginRows(ctx, sessionToken, "PluginFormcreatorSection", 2000, &all); err != nil {
		return nil, err
	}
	out := make([]FormcreatorSection, 0, 8)
	for _, s := range all {
		if int(s.FormsID) == formID {
			out = append(out, s)
		}
	}
	return out, nil
}

// ListFormcreatorQuestionsByForm returns every question on a given form.
func (c *Client) ListFormcreatorQuestionsByForm(ctx context.Context, sessionToken string, formID int) ([]FormcreatorQuestion, error) {
	// Questions belong to a section, not directly to a form in some
	// Formcreator versions — others keep the plugin_formcreator_forms_id FK
	// denormalized on the question row. We resolve the sections first, then
	// keep any question whose section id OR plugin_formcreator_forms_id (if
	// present) points at this form.
	sections, err := c.ListFormcreatorSectionsByForm(ctx, sessionToken, formID)
	if err != nil {
		return nil, err
	}
	sectionIDs := make(map[int]struct{}, len(sections))
	for _, s := range sections {
		sectionIDs[int(s.ID)] = struct{}{}
	}

	var all []FormcreatorQuestion
	if err := c.fetchAllPluginRows(ctx, sessionToken, "PluginFormcreatorQuestion", 5000, &all); err != nil {
		return nil, err
	}
	out := make([]FormcreatorQuestion, 0, 32)
	for _, q := range all {
		if _, ok := sectionIDs[int(q.SectionsID)]; ok {
			out = append(out, q)
		}
	}
	return out, nil
}

// ListFormcreatorConditionsByForm returns both question- and section-scoped
// conditions that belong to this form. Conditions reference items_id pointing
// at a question or section id; we keep the rows whose target is in the form's
// set of sections/questions.
func (c *Client) ListFormcreatorConditionsByForm(ctx context.Context, sessionToken string, formID int) ([]FormcreatorCondition, error) {
	sections, err := c.ListFormcreatorSectionsByForm(ctx, sessionToken, formID)
	if err != nil {
		return nil, err
	}
	questions, err := c.ListFormcreatorQuestionsByForm(ctx, sessionToken, formID)
	if err != nil {
		return nil, err
	}
	sectionIDs := make(map[int]struct{}, len(sections))
	for _, s := range sections {
		sectionIDs[int(s.ID)] = struct{}{}
	}
	questionIDs := make(map[int]struct{}, len(questions))
	for _, q := range questions {
		questionIDs[int(q.ID)] = struct{}{}
	}

	var all []FormcreatorCondition
	if err := c.fetchAllPluginRows(ctx, sessionToken, "PluginFormcreatorCondition", 5000, &all); err != nil {
		return nil, err
	}
	out := make([]FormcreatorCondition, 0, 16)
	for _, c := range all {
		switch c.ItemType {
		case "PluginFormcreatorSection":
			if _, ok := sectionIDs[int(c.ItemsID)]; ok {
				out = append(out, c)
			}
		case "PluginFormcreatorQuestion":
			if _, ok := questionIDs[int(c.ItemsID)]; ok {
				out = append(out, c)
			}
		}
	}
	return out, nil
}

// ListFormcreatorTags returns every tag the profile can see. Formcreator tags
// are scoped to the whole plugin (not per form), so the picker can cache the
// list and filter client-side. Optional query narrows the server call when
// large instances have thousands of tags.
func (c *Client) ListFormcreatorTags(ctx context.Context, sessionToken, query string, rangeStart, rangeEnd int) ([]map[string]any, error) {
	q := url.Values{}
	if query != "" {
		q.Set("criteria[0][field]", "1")
		q.Set("criteria[0][searchtype]", "contains")
		q.Set("criteria[0][value]", query)
	}
	q.Set("forcedisplay[0]", "2") // id
	q.Set("forcedisplay[1]", "1") // name
	q.Set("forcedisplay[2]", "3") // color (hex)
	q.Set("range", fmt.Sprintf("%d-%d", rangeStart, rangeEnd))
	var raw struct {
		Data []map[string]any `json:"data"`
	}
	if err := c.do(ctx, "GET", "/search/PluginFormcreatorTag", sessionToken, q, nil, &raw); err != nil {
		return nil, err
	}
	return raw.Data, nil
}

// SubmitFormcreatorFormAnswer posts the final answer payload. values is a
// clean `{questionID: value}` map — we wrap it into the
// `formcreator_field_<id>` shape Formcreator's controller expects.
// Returns the FormAnswer id + status (101 waiting, 102 accepted, 103 refused).
func (c *Client) SubmitFormcreatorFormAnswer(ctx context.Context, sessionToken string, formID int, values map[string]any) (*FormcreatorSubmitResult, error) {
	input := map[string]any{
		"plugin_formcreator_forms_id": formID,
	}
	for qid, val := range values {
		input[fmt.Sprintf("formcreator_field_%s", qid)] = val
	}
	body := map[string]any{"input": input}

	// Formcreator returns {"id":N, "plugin_formcreator_forms_id":N, ...} on
	// success; some versions return the id directly as a bare int. Decode into
	// a RawMessage first so we can accept either shape.
	var raw json.RawMessage
	if err := c.do(ctx, "POST", "/PluginFormcreatorFormAnswer", sessionToken, nil, body, &raw); err != nil {
		return nil, err
	}

	result := &FormcreatorSubmitResult{}
	// Shape 1: bare id (older Formcreator).
	var asInt int
	if err := json.Unmarshal(raw, &asInt); err == nil && asInt > 0 {
		result.FormAnswerID = asInt
		return result, nil
	}
	// Shape 2: full object.
	var asObj struct {
		ID     int `json:"id"`
		Status int `json:"status"`
	}
	if err := json.Unmarshal(raw, &asObj); err != nil {
		return nil, fmt.Errorf("parse form-answer response: %w (raw=%s)", err, truncate(string(raw), 200))
	}
	result.FormAnswerID = asObj.ID
	result.Status = asObj.Status
	return result, nil
}
