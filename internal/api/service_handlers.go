package api

import (
	"net/http"

	"github.com/gabrielfmcoelho/ssh-config-manager/internal/database"
	"github.com/gabrielfmcoelho/ssh-config-manager/internal/models"
)

type serviceHandlers struct {
	db *database.DB
}

func (h *serviceHandlers) handleList(w http.ResponseWriter, r *http.Request) {
	services, err := models.ListServices(h.db.SQL)
	if err != nil {
		jsonServerError(w, r, "failed to list services", err)
		return
	}

	tagMap, _ := models.GetAllTags(h.db.SQL, "service")
	mainRespNames, _ := models.GetServiceMainResponsavelNamesBulk(h.db.SQL)
	type serviceWithMeta struct {
		models.Service
		Tags                  []string `json:"tags"`
		HostIDs               []int64  `json:"host_ids"`
		DNSIDs                []int64  `json:"dns_ids"`
		DependsOnIDs          []int64  `json:"depends_on_ids"`
		MainResponsavelName   string   `json:"main_responsavel_name"`
	}
	result := make([]serviceWithMeta, len(services))
	for i, svc := range services {
		hostIDs, _ := models.GetServiceHostIDs(h.db.SQL, svc.ID)
		dnsIDs, _ := models.GetServiceDNSIDs(h.db.SQL, svc.ID)
		depIDs, _ := models.GetServiceDependencyIDs(h.db.SQL, svc.ID)
		result[i] = serviceWithMeta{
			Service:             svc,
			Tags:                tagMap[svc.ID],
			HostIDs:             hostIDs,
			DNSIDs:              dnsIDs,
			DependsOnIDs:        depIDs,
			MainResponsavelName: mainRespNames[svc.ID],
		}
	}

	jsonOK(w, result)
}

func (h *serviceHandlers) handleGet(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid id", err)
		return
	}

	svc, err := models.GetService(h.db.SQL, id)
	if err != nil || svc == nil {
		jsonError(w, http.StatusNotFound, "service not found")
		return
	}

	tags, _ := models.GetTags(h.db.SQL, "service", id)
	hostIDs, _ := models.GetServiceHostIDs(h.db.SQL, id)
	dnsIDs, _ := models.GetServiceDNSIDs(h.db.SQL, id)
	dependsOnIDs, _ := models.GetServiceDependencyIDs(h.db.SQL, id)
	dependentIDs, _ := models.GetServiceDependentIDs(h.db.SQL, id)

	// List credentials (role names only).
	creds, _ := models.ListServiceCredentials(h.db.SQL, id)
	type credSummary struct {
		ID       int64  `json:"id"`
		RoleName string `json:"role_name"`
	}
	credList := make([]credSummary, len(creds))
	for i, c := range creds {
		credList[i] = credSummary{ID: c.ID, RoleName: c.RoleName}
	}

	responsaveis, _ := models.ListServiceResponsaveis(h.db.SQL, id)

	jsonOK(w, map[string]any{
		"service":        svc,
		"tags":           tags,
		"host_ids":       hostIDs,
		"dns_ids":        dnsIDs,
		"depends_on_ids": dependsOnIDs,
		"dependent_ids":  dependentIDs,
		"credentials":    credList,
		"responsaveis":   responsaveis,
	})
}

func (h *serviceHandlers) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		models.Service
		Tags         []string                          `json:"tags"`
		HostIDs      []int64                           `json:"host_ids"`
		DNSIDs       []int64                           `json:"dns_ids"`
		DependsOnIDs []int64                           `json:"depends_on_ids"`
		Responsaveis []models.ServiceResponsavelInput  `json:"responsaveis"`
	}
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid request body", err)
		return
	}
	if req.Nickname == "" {
		jsonError(w, http.StatusBadRequest, "nickname is required")
		return
	}

	if err := models.CreateService(h.db.SQL, &req.Service); err != nil {
		jsonServerError(w, r, "failed to create service", err)
		return
	}

	if len(req.Tags) > 0 {
		models.SetTags(h.db.SQL, "service", req.Service.ID, req.Tags)
	}
	if len(req.HostIDs) > 0 {
		models.SetServiceHostLinks(h.db.SQL, req.Service.ID, req.HostIDs)
	}
	if len(req.DNSIDs) > 0 {
		models.SetServiceDNSLinks(h.db.SQL, req.Service.ID, req.DNSIDs)
	}
	if len(req.DependsOnIDs) > 0 {
		models.SetServiceDependencies(h.db.SQL, req.Service.ID, req.DependsOnIDs)
	}
	if len(req.Responsaveis) > 0 {
		models.SyncServiceResponsaveis(h.db.SQL, req.Service.ID, req.Responsaveis)
	}

	jsonCreated(w, req.Service)
}

func (h *serviceHandlers) handleUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid id", err)
		return
	}

	existing, err := models.GetService(h.db.SQL, id)
	if err != nil || existing == nil {
		jsonError(w, http.StatusNotFound, "service not found")
		return
	}

	var req struct {
		models.Service
		Tags         []string                            `json:"tags"`
		HostIDs      []int64                             `json:"host_ids"`
		DNSIDs       []int64                             `json:"dns_ids"`
		DependsOnIDs []int64                             `json:"depends_on_ids"`
		Responsaveis *[]models.ServiceResponsavelInput   `json:"responsaveis"`
	}
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid request body", err)
		return
	}

	req.Service.ID = id
	if err := models.UpdateService(h.db.SQL, &req.Service); err != nil {
		jsonServerError(w, r, "failed to update service", err)
		return
	}

	if req.Tags != nil {
		models.SetTags(h.db.SQL, "service", id, req.Tags)
	}
	if req.HostIDs != nil {
		models.SetServiceHostLinks(h.db.SQL, id, req.HostIDs)
	}
	if req.DNSIDs != nil {
		models.SetServiceDNSLinks(h.db.SQL, id, req.DNSIDs)
	}
	if req.DependsOnIDs != nil {
		models.SetServiceDependencies(h.db.SQL, id, req.DependsOnIDs)
	}
	if req.Responsaveis != nil {
		models.SyncServiceResponsaveis(h.db.SQL, id, *req.Responsaveis)
	}

	jsonOK(w, req.Service)
}

func (h *serviceHandlers) handleDelete(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid id", err)
		return
	}

	models.DeleteTags(h.db.SQL, "service", id)
	if err := models.DeleteService(h.db.SQL, id); err != nil {
		jsonServerError(w, r, "failed to delete service", err)
		return
	}
	jsonOK(w, map[string]string{"status": "deleted"})
}

// Service credentials

func (h *serviceHandlers) handleListCredentials(w http.ResponseWriter, r *http.Request) {
	serviceID, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid service id", err)
		return
	}

	creds, err := models.ListServiceCredentials(h.db.SQL, serviceID)
	if err != nil {
		jsonServerError(w, r, "failed to list credentials", err)
		return
	}

	// Return role names only (not decrypted secrets).
	type credSummary struct {
		ID       int64  `json:"id"`
		RoleName string `json:"role_name"`
	}
	result := make([]credSummary, len(creds))
	for i, c := range creds {
		result[i] = credSummary{ID: c.ID, RoleName: c.RoleName}
	}
	jsonOK(w, result)
}

func (h *serviceHandlers) handleCreateCredential(w http.ResponseWriter, r *http.Request) {
	serviceID, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid service id", err)
		return
	}

	var req struct {
		RoleName    string `json:"role_name"`
		Credentials string `json:"credentials"`
	}
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid request body", err)
		return
	}
	if req.RoleName == "" || req.Credentials == "" {
		jsonError(w, http.StatusBadRequest, "role_name and credentials are required")
		return
	}

	ct, nonce, err := h.db.Encryptor.Encrypt(req.Credentials)
	if err != nil {
		jsonServerError(w, r, "failed to encrypt credentials", err)
		return
	}

	sc := &models.ServiceCredential{
		ServiceID:             serviceID,
		RoleName:              req.RoleName,
		CredentialsCiphertext: ct,
		CredentialsNonce:      nonce,
	}
	if err := models.CreateServiceCredential(h.db.SQL, sc); err != nil {
		jsonError(w, http.StatusConflict, "credential role already exists for this service")
		return
	}

	jsonCreated(w, map[string]any{"id": sc.ID, "role_name": sc.RoleName})
}

func (h *serviceHandlers) handleGetCredential(w http.ResponseWriter, r *http.Request) {
	credID, err := pathInt64(r, "credId")
	if err != nil {
		jsonBadRequest(w, r, "invalid credential id", err)
		return
	}

	cred, err := models.GetServiceCredential(h.db.SQL, credID)
	if err != nil || cred == nil {
		jsonError(w, http.StatusNotFound, "credential not found")
		return
	}

	plaintext, err := h.db.Encryptor.Decrypt(cred.CredentialsCiphertext, cred.CredentialsNonce)
	if err != nil {
		jsonServerError(w, r, "failed to decrypt credentials", err)
		return
	}

	jsonOK(w, map[string]any{
		"id":          cred.ID,
		"role_name":   cred.RoleName,
		"credentials": plaintext,
	})
}

func (h *serviceHandlers) handleDeleteCredential(w http.ResponseWriter, r *http.Request) {
	credID, err := pathInt64(r, "credId")
	if err != nil {
		jsonBadRequest(w, r, "invalid credential id", err)
		return
	}

	if err := models.DeleteServiceCredential(h.db.SQL, credID); err != nil {
		jsonServerError(w, r, "failed to delete credential", err)
		return
	}
	jsonOK(w, map[string]string{"status": "deleted"})
}

// handleFixate converts an auto-discovered service to a fixed service.
func (h *serviceHandlers) handleFixate(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid id", err)
		return
	}

	svc, err := models.GetService(h.db.SQL, id)
	if err != nil || svc == nil {
		jsonError(w, http.StatusNotFound, "service not found")
		return
	}
	if svc.Source != "auto" {
		jsonError(w, http.StatusBadRequest, "only auto-discovered services can be fixated")
		return
	}

	if err := models.FixateService(h.db.SQL, id); err != nil {
		jsonServerError(w, r, "failed to fixate service", err)
		return
	}

	svc.Source = "fixed"
	jsonOK(w, svc)
}

// handleUpdateContainer rebinds a fixed/manual service to a different container.
func (h *serviceHandlers) handleUpdateContainer(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		jsonBadRequest(w, r, "invalid id", err)
		return
	}

	svc, err := models.GetService(h.db.SQL, id)
	if err != nil || svc == nil {
		jsonError(w, http.StatusNotFound, "service not found")
		return
	}
	if svc.Source == "auto" {
		jsonError(w, http.StatusBadRequest, "cannot rebind auto services; fixate first")
		return
	}

	var req struct {
		ContainerName string `json:"container_name"`
		ContainerID   string `json:"container_id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		jsonBadRequest(w, r, "invalid request body", err)
		return
	}

	if err := models.UpdateContainerBinding(h.db.SQL, id, req.ContainerName, req.ContainerID); err != nil {
		jsonServerError(w, r, "failed to update container binding", err)
		return
	}

	svc.ContainerName = req.ContainerName
	svc.ContainerID = req.ContainerID
	jsonOK(w, svc)
}

// handleListAllCredentials returns all services that have credentials, with role summaries.
func (h *serviceHandlers) handleListAllCredentials(w http.ResponseWriter, r *http.Request) {
	services, err := models.ListServices(h.db.SQL)
	if err != nil {
		jsonServerError(w, r, "failed to list services", err)
		return
	}

	type credSummary struct {
		ID       int64  `json:"id"`
		RoleName string `json:"role_name"`
	}
	type serviceWithCreds struct {
		ServiceID       int64         `json:"service_id"`
		ServiceNickname string        `json:"service_nickname"`
		ServiceType     string        `json:"service_type"`
		Credentials     []credSummary `json:"credentials"`
	}

	var result []serviceWithCreds
	for _, svc := range services {
		creds, err := models.ListServiceCredentials(h.db.SQL, svc.ID)
		if err != nil {
			jsonServerError(w, r, "failed to list credentials", err)
			return
		}
		if len(creds) == 0 {
			continue
		}
		summaries := make([]credSummary, len(creds))
		for i, c := range creds {
			summaries[i] = credSummary{ID: c.ID, RoleName: c.RoleName}
		}
		result = append(result, serviceWithCreds{
			ServiceID:       svc.ID,
			ServiceNickname: svc.Nickname,
			ServiceType:     svc.ServiceType,
			Credentials:     summaries,
		})
	}
	if result == nil {
		result = []serviceWithCreds{}
	}
	jsonOK(w, result)
}
