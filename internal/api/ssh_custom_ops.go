package api

import (
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/sshtest"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/store"
)

// customOpPlaceholder matches a {{name}} parameter in a custom operation's
// script. The frontend twin is scriptParams in lib/sshCustomOps.ts.
var customOpPlaceholder = regexp.MustCompile(`\{\{([a-z_][a-z0-9_]*)\}\}`)

// customOpTimeout bounds one run: the client is closed when it fires.
// ponytail: fixed for every operation; make it per-operation if a script needs more.
const customOpTimeout = 10 * time.Minute

// shellQuote wraps v in single quotes so the shell reads it as one literal word.
func shellQuote(v string) string {
	return "'" + strings.ReplaceAll(v, "'", `'\''`) + "'"
}

// renderScript fills every {{name}} with its shell-quoted value. Values come
// from editors and the script may run under sudo, so quoting is what keeps a
// value from adding commands. Every placeholder needs a non-empty value.
func renderScript(script string, params map[string]string) (string, error) {
	var missing []string
	for _, m := range customOpPlaceholder.FindAllStringSubmatch(script, -1) {
		if strings.TrimSpace(params[m[1]]) == "" && !slices.Contains(missing, m[1]) {
			missing = append(missing, m[1])
		}
	}
	if len(missing) > 0 {
		return "", fmt.Errorf("missing value for: %s", strings.Join(missing, ", "))
	}
	return customOpPlaceholder.ReplaceAllStringFunc(script, func(p string) string {
		return shellQuote(strings.TrimSpace(params[p[2:len(p)-2]]))
	}), nil
}

func (h *sshHandlers) customOps() *store.SSHCustomOperationRepo {
	return store.NewSSHCustomOperationRepo(h.db.SQL)
}

// applyCustomOpInput merges the partial input over o and checks the result.
func applyCustomOpInput(o *models.SSHCustomOperation, in models.SSHCustomOperationInput) error {
	if in.Name != nil {
		o.Name = strings.TrimSpace(*in.Name)
	}
	if in.Description != nil {
		o.Description = strings.TrimSpace(*in.Description)
	}
	if in.Script != nil {
		o.Script = *in.Script
	}
	if in.UseSudo != nil {
		o.UseSudo = *in.UseSudo
	}
	if o.Name == "" || strings.TrimSpace(o.Script) == "" {
		return fmt.Errorf("name and script are required")
	}
	return nil
}

// loadCustomOp reads the {id} path value and its operation, or writes 400/404.
func (h *sshHandlers) loadCustomOp(w http.ResponseWriter, r *http.Request) *models.SSHCustomOperation {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid id", err)
		return nil
	}
	o, err := h.customOps().Get(r.Context(), id)
	if err != nil {
		jsonServerError(w, r, "lookup failed", err)
		return nil
	}
	if o == nil {
		jsonError(w, http.StatusNotFound, "custom operation not found")
		return nil
	}
	return o
}

// saveCustomOp writes o (create when ID is 0) and answers 409 on a taken name.
func (h *sshHandlers) saveCustomOp(w http.ResponseWriter, r *http.Request, o *models.SSHCustomOperation) bool {
	var err error
	if o.ID == 0 {
		err = h.customOps().Create(r.Context(), o)
	} else {
		err = h.customOps().Update(r.Context(), o)
	}
	if store.IsUniqueViolation(err) {
		jsonError(w, http.StatusConflict, "a custom operation with this name already exists")
		return false
	}
	if err != nil {
		jsonServerError(w, r, "save failed", err)
		return false
	}
	return true
}

// handleListCustomOps godoc
//
//	@Summary		List custom SSH operations
//	@Description	Editor. The admin-written scripts runnable from a host's Operações tab.
//	@Tags			ssh
//	@Produce		json
//	@Param			page		query		int	false	"Page (1-based)"
//	@Param			per_page	query		int	false	"Page size (max 200); omit for every row"
//	@Success		200			{object}	ListEnvelope[models.SSHCustomOperation]
//	@Failure		401			{object}	httpx.ErrorResponse
//	@Failure		403			{object}	httpx.ErrorResponse
//	@Router			/api/ssh/custom-operations [get]
func (h *sshHandlers) handleListCustomOps(w http.ResponseWriter, r *http.Request) {
	list, err := h.customOps().List(r.Context())
	if err != nil {
		jsonServerError(w, r, "failed to list custom operations", err)
		return
	}
	jsonPaged(w, r, list)
}

// handleCreateCustomOp godoc
//
//	@Summary		Create a custom SSH operation
//	@Description	Admin. name and script are required; double-brace placeholders in the script (name inside two pairs of curly braces) are asked for at run time.
//	@Tags			ssh
//	@Accept			json
//	@Produce		json
//	@Param			body	body		models.SSHCustomOperationInput	true	"Operation"
//	@Success		201		{object}	models.SSHCustomOperation
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Failure		409		{object}	httpx.ErrorResponse
//	@Router			/api/ssh/custom-operations [post]
func (h *sshHandlers) handleCreateCustomOp(w http.ResponseWriter, r *http.Request) {
	var in models.SSHCustomOperationInput
	if err := decodeJSON(r, &in); err != nil {
		jsonBadRequest(w, r, "invalid JSON", err)
		return
	}
	o := models.SSHCustomOperation{}
	if err := applyCustomOpInput(&o, in); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	if uid := actorID(r); uid != 0 {
		o.CreatedBy = &uid
	}
	if h.saveCustomOp(w, r, &o) {
		jsonCreated(w, o)
	}
}

// handleUpdateCustomOp godoc
//
//	@Summary		Update a custom SSH operation
//	@Description	Admin. Partial: an omitted field keeps its value.
//	@Tags			ssh
//	@Accept			json
//	@Produce		json
//	@Param			id		path		int								true	"Operation ID"
//	@Param			body	body		models.SSHCustomOperationInput	true	"Fields to change"
//	@Success		200		{object}	models.SSHCustomOperation
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Failure		409		{object}	httpx.ErrorResponse
//	@Router			/api/ssh/custom-operations/{id} [put]
func (h *sshHandlers) handleUpdateCustomOp(w http.ResponseWriter, r *http.Request) {
	o := h.loadCustomOp(w, r)
	if o == nil {
		return
	}
	var in models.SSHCustomOperationInput
	if err := decodeJSON(r, &in); err != nil {
		jsonBadRequest(w, r, "invalid JSON", err)
		return
	}
	if err := applyCustomOpInput(o, in); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	if h.saveCustomOp(w, r, o) {
		jsonOK(w, o)
	}
}

// handleDeleteCustomOp godoc
//
//	@Summary		Delete a custom SSH operation
//	@Description	Admin. Its past runs stay in the hosts' operation history.
//	@Tags			ssh
//	@Produce		json
//	@Param			id	path		int	true	"Operation ID"
//	@Success		200	{object}	StatusResponse
//	@Failure		400	{object}	httpx.ErrorResponse
//	@Failure		401	{object}	httpx.ErrorResponse
//	@Failure		403	{object}	httpx.ErrorResponse
//	@Router			/api/ssh/custom-operations/{id} [delete]
func (h *sshHandlers) handleDeleteCustomOp(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid id", err)
		return
	}
	if err := h.customOps().Delete(r.Context(), id); err != nil {
		jsonServerError(w, r, "delete failed", err)
		return
	}
	jsonOK(w, map[string]string{"status": "deleted"})
}

// handleRunCustomOp godoc
//
//	@Summary		Run a custom SSH operation on a host
//	@Description	Editor. Body {"params": {name: value}} fills the script's double-brace placeholders, each shell-quoted. Operations with use_sudo need the host's stored password. Runs for at most 10 minutes; logged in the host's operation history as custom:<name>.
//	@Tags			ssh
//	@Accept			json
//	@Produce		json
//	@Param			id		path		int								true	"Operation ID"
//	@Param			slug	path		string							true	"Host slug"
//	@Param			body	body		models.SSHCustomOperationRun	false	"Parameter values"
//	@Success		200		{object}	map[string]any
//	@Failure		400		{object}	httpx.ErrorResponse
//	@Failure		401		{object}	httpx.ErrorResponse
//	@Failure		403		{object}	httpx.ErrorResponse
//	@Failure		404		{object}	httpx.ErrorResponse
//	@Failure		502		{object}	httpx.ErrorResponse
//	@Router			/api/ssh/custom-operations/{id}/run/{slug} [post]
func (h *sshHandlers) handleRunCustomOp(w http.ResponseWriter, r *http.Request) {
	host := h.requireHost(w, r)
	if host == nil {
		return
	}
	op := h.loadCustomOp(w, r)
	if op == nil {
		return
	}
	var in models.SSHCustomOperationRun
	if r.ContentLength != 0 {
		if err := decodeJSON(r, &in); err != nil {
			jsonBadRequest(w, r, "invalid JSON", err)
			return
		}
	}
	script, err := renderScript(op.Script, in.Params)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	user := h.requireUser(w, host)
	if user == "" {
		return
	}
	method, auth, ok := h.resolveAuth(w, host, "")
	if !ok {
		return
	}
	if op.UseSudo && auth.Password() == "" {
		jsonError(w, http.StatusBadRequest, "stored password required for this operation (uses sudo)")
		return
	}
	client := h.dial(w, host, user, auth)
	if client == nil {
		return
	}
	defer client.Close()

	timer := time.AfterFunc(customOpTimeout, func() { client.Close() })
	var output string
	var runErr error
	if op.UseSudo {
		output, runErr = sshtest.RunPrivilegedScript(client, auth.Password(), script)
	} else {
		output, runErr = sshtest.RunScript(client, script)
	}
	timedOut := !timer.Stop()

	opType := "custom:" + op.Name
	logged := paramsLine(in.Params) + output
	if runErr != nil {
		errMsg := runErr.Error()
		if timedOut {
			errMsg = fmt.Sprintf("timed out after %s", customOpTimeout)
		} else if output != "" {
			errMsg = output
		}
		h.logOperation(r, host.ID, opType, &method, "failed", paramsLine(in.Params)+errMsg)
		jsonOK(w, map[string]any{"success": false, "error": errMsg, "output": output})
		return
	}
	h.logOperation(r, host.ID, opType, &method, "success", logged)
	jsonOK(w, map[string]any{"success": true, "output": output})
}

// paramsLine records the values a run used, for the operation history.
func paramsLine(params map[string]string) string {
	if len(params) == 0 {
		return ""
	}
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = k + "=" + shellQuote(params[k])
	}
	return "params: " + strings.Join(parts, " ") + "\n\n"
}
