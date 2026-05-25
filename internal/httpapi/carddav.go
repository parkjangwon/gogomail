package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gogomail/gogomail/internal/apikeys"
	"github.com/gogomail/gogomail/internal/auth"
	"github.com/gogomail/gogomail/internal/carddavgw"
	"github.com/gogomail/gogomail/internal/directory"
	"github.com/gogomail/gogomail/internal/orgchart"
)

type ContactRepo interface {
	ListAddressBooks(ctx context.Context, req carddavgw.ListAddressBooksRequest) ([]carddavgw.AddressBook, error)
	CreateAddressBook(ctx context.Context, req carddavgw.CreateAddressBookRequest) (carddavgw.AddressBook, error)
	GetAddressBook(ctx context.Context, req carddavgw.GetAddressBookRequest) (carddavgw.AddressBook, error)
	UpdateAddressBookProperties(ctx context.Context, req carddavgw.UpdateAddressBookRequest) (carddavgw.AddressBook, error)
	DeleteAddressBook(ctx context.Context, req carddavgw.DeleteAddressBookRequest) (carddavgw.AddressBook, error)
	ListContactObjects(ctx context.Context, req carddavgw.ListContactObjectsRequest) ([]carddavgw.ContactObject, error)
	GetContactObject(ctx context.Context, req carddavgw.GetContactObjectRequest) (carddavgw.ContactObject, error)
	UpsertContactObject(ctx context.Context, req carddavgw.UpsertContactObjectRequest) (carddavgw.ContactObject, error)
	DeleteContactObject(ctx context.Context, req carddavgw.DeleteContactObjectRequest) (carddavgw.ContactObject, error)
	SearchContacts(ctx context.Context, req carddavgw.SearchContactsRequest) ([]carddavgw.ContactObject, error)
}

type DirectoryRepo interface {
	SearchPrincipals(ctx context.Context, req directory.SearchPrincipalsRequest) ([]directory.Principal, error)
	ResolvePrincipal(ctx context.Context, req directory.ResolvePrincipalRequest) (directory.Principal, error)
	ResolveUserByEmail(ctx context.Context, req directory.ResolveUserByEmailRequest) (directory.Principal, error)
	ListOrgTree(ctx context.Context, companyID, domainID string) ([]directory.OrgTreeItem, error)
	ListOrgMembersByOrgIDs(ctx context.Context, companyID, domainID string, orgIDs []string, limitPerOrg int) (map[string][]directory.Principal, error)
}

// OrgProfiler resolves org memberships (unit name + title) for a user.
type OrgProfiler interface {
	GetMembershipsForUser(ctx context.Context, userID string) ([]orgchart.MembershipDetail, error)
}

// DirectoryUserResolver resolves a user principal by email, scoped to a company/domain.
type DirectoryUserResolver interface {
	ResolveUserByEmail(ctx context.Context, req directory.ResolveUserByEmailRequest) (directory.Principal, error)
}

type ContactHandler struct {
	repo          ContactRepo
	directoryRepo DirectoryRepo
	orgProfiler   OrgProfiler
}

func NewContactHandler(repo ContactRepo, dirRepo DirectoryRepo) *ContactHandler {
	return &ContactHandler{repo: repo, directoryRepo: dirRepo}
}

// WithOrgProfiler attaches an org profiler so the directory profile endpoint can return
// org unit name and title for internal users.
func (h *ContactHandler) WithOrgProfiler(op OrgProfiler) *ContactHandler {
	h.orgProfiler = op
	return h
}

type AddressBookEnvelope struct {
	AddressBook carddavgw.AddressBook `json:"address_book"`
}

type AddressBookListEnvelope struct {
	AddressBooks []carddavgw.AddressBook `json:"address_books"`
}

type ContactObjectEnvelope struct {
	Contact carddavgw.ContactObject `json:"contact"`
}

type ContactObjectListEnvelope struct {
	Contacts []carddavgw.ContactObject `json:"contacts"`
}

type AutocompleteResult struct {
	Type          string `json:"type"`
	ID            string `json:"id"`
	DisplayName   string `json:"display_name"`
	Email         string `json:"email"`
	AddressBookID string `json:"address_book_id,omitempty"`
}

type AutocompleteResponse struct {
	Results []AutocompleteResult `json:"results"`
}

func RegisterContactRoutes(mux *http.ServeMux, handler *ContactHandler, tokenManager *auth.TokenManager) {
	allows := []string{}
	if tokenManager == nil {
		allows = []string{"user_id"}
	}

	mux.HandleFunc("GET /api/mail/addressbooks", func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r, allows...) {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		addressBooks, err := handler.repo.ListAddressBooks(r.Context(), carddavgw.ListAddressBooksRequest{UserID: userID})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "address book list failed")
			return
		}
		writeJSON(w, http.StatusOK, AddressBookListEnvelope{AddressBooks: addressBooks})
	})

	mux.HandleFunc("POST /api/mail/addressbooks", func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, allows...) {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		if r.ContentLength > maxJSONBodyBytes {
			writeError(w, http.StatusRequestEntityTooLarge, "request body too large")
			return
		}
		var req carddavgw.CreateAddressBookRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("decode address book create request: %w", err).Error())
			return
		}
		req.UserID = userID
		addressBook, err := handler.repo.CreateAddressBook(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("create address book: %w", err).Error())
			return
		}
		w.Header().Set("Location", fmt.Sprintf("/api/mail/addressbooks/%s", addressBook.ID))
		writeJSON(w, http.StatusCreated, AddressBookEnvelope{AddressBook: addressBook})
	})

	mux.HandleFunc("GET /api/mail/addressbooks/{id}", func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r, allows...) {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		addressBookID := r.PathValue("id")
		if addressBookID == "" {
			writeError(w, http.StatusBadRequest, "address book id is required")
			return
		}
		addressBook, err := handler.repo.GetAddressBook(r.Context(), carddavgw.GetAddressBookRequest{UserID: userID, AddressBookID: addressBookID})
		if err != nil {
			writeError(w, http.StatusNotFound, "address book not found")
			return
		}
		writeJSON(w, http.StatusOK, AddressBookEnvelope{AddressBook: addressBook})
	})

	mux.HandleFunc("PATCH /api/mail/addressbooks/{id}", func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, allows...) {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		addressBookID := r.PathValue("id")
		if addressBookID == "" {
			writeError(w, http.StatusBadRequest, "address book id is required")
			return
		}
		if r.ContentLength > maxJSONBodyBytes {
			writeError(w, http.StatusRequestEntityTooLarge, "request body too large")
			return
		}
		var req carddavgw.UpdateAddressBookRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("decode address book update request: %w", err).Error())
			return
		}
		req.UserID = userID
		req.AddressBookID = addressBookID
		addressBook, err := handler.repo.UpdateAddressBookProperties(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("update address book: %w", err).Error())
			return
		}
		writeJSON(w, http.StatusOK, AddressBookEnvelope{AddressBook: addressBook})
	})

	mux.HandleFunc("DELETE /api/mail/addressbooks/{id}", func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r, allows...) {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		addressBookID := r.PathValue("id")
		if addressBookID == "" {
			writeError(w, http.StatusBadRequest, "address book id is required")
			return
		}
		etag := r.Header.Get("If-Match")
		_, err := handler.repo.DeleteAddressBook(r.Context(), carddavgw.DeleteAddressBookRequest{UserID: userID, AddressBookID: addressBookID, ObservedETag: etag})
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("delete address book: %w", err).Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("GET /api/mail/addressbooks/{id}/contacts", func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r, allows...) {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		addressBookID := r.PathValue("id")
		if addressBookID == "" {
			writeError(w, http.StatusBadRequest, "address book id is required")
			return
		}
		objects, err := handler.repo.ListContactObjects(r.Context(), carddavgw.ListContactObjectsRequest{UserID: userID, AddressBookID: addressBookID})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "contact list failed")
			return
		}
		writeJSON(w, http.StatusOK, ContactObjectListEnvelope{Contacts: objects})
	})

	mux.HandleFunc("GET /api/mail/addressbooks/{id}/contacts/{name}", func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r, allows...) {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		addressBookID := r.PathValue("id")
		objectName := r.PathValue("name")
		if addressBookID == "" || objectName == "" {
			writeError(w, http.StatusBadRequest, "address book id and contact name are required")
			return
		}
		object, err := handler.repo.GetContactObject(r.Context(), carddavgw.GetContactObjectRequest{UserID: userID, AddressBookID: addressBookID, ObjectName: objectName})
		if err != nil {
			writeError(w, http.StatusNotFound, "contact not found")
			return
		}
		if matchETag(r, object.ETag) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("Content-Type", "text/vcard; charset=utf-8")
		w.Header().Set("ETag", fmt.Sprintf(`"%s"`, object.ETag))
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		w.Write(object.VCard)
	})

	mux.HandleFunc("PUT /api/mail/addressbooks/{id}/contacts/{name}", func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, allows...) {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		addressBookID := r.PathValue("id")
		objectName := r.PathValue("name")
		if addressBookID == "" || objectName == "" {
			writeError(w, http.StatusBadRequest, "address book id and contact name are required")
			return
		}
		contentType := r.Header.Get("Content-Type")
		if contentType != "text/vcard" && contentType != "application/vcard+xml" && contentType != "text/x-vcard" {
			writeError(w, http.StatusUnsupportedMediaType, "Content-Type must be text/vcard, application/vcard+xml, or text/x-vcard")
			return
		}
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("read request body: %w", err).Error())
			return
		}
		etag := r.Header.Get("If-Match")
		object, err := handler.repo.UpsertContactObject(r.Context(), carddavgw.UpsertContactObjectRequest{
			UserID:        userID,
			AddressBookID: addressBookID,
			ObjectName:    objectName,
			VCard:         body,
			ObservedETag:  etag,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("upsert contact: %w", err).Error())
			return
		}
		w.Header().Set("ETag", fmt.Sprintf(`"%s"`, object.ETag))
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusOK, ContactObjectEnvelope{Contact: object})
	})

	mux.HandleFunc("DELETE /api/mail/addressbooks/{id}/contacts/{name}", func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r, allows...) {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		addressBookID := r.PathValue("id")
		objectName := r.PathValue("name")
		if addressBookID == "" || objectName == "" {
			writeError(w, http.StatusBadRequest, "address book id and contact name are required")
			return
		}
		etag := r.Header.Get("If-Match")
		_, err := handler.repo.DeleteContactObject(r.Context(), carddavgw.DeleteContactObjectRequest{
			UserID:        userID,
			AddressBookID: addressBookID,
			ObjectName:    objectName,
			ObservedETag:  etag,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("delete contact: %w", err).Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("GET /api/mail/contacts/autocomplete", func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		autocompleteAllows := append(append([]string(nil), allows...), "q", "limit")
		if !rejectUnknownQueryKeys(w, r, autocompleteAllows...) {
			return
		}

		var userID, domainID string
		if tokenManager != nil {
			claims, ok := claimsFromRequest(w, r, tokenManager)
			if !ok {
				return
			}
			userID = claims.UserID
			domainID = claims.DomainID
		} else {
			var ok bool
			userID, ok = userIDFromRequest(w, r, nil)
			if !ok {
				return
			}
		}

		q := r.URL.Query().Get("q")
		if q == "" {
			writeError(w, http.StatusBadRequest, "q parameter is required")
			return
		}
		if len(q) > 200 {
			writeError(w, http.StatusBadRequest, "q parameter is too long")
			return
		}
		limit := 10
		if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
			if n, ok := parsePositiveInt(limitStr); ok {
				limit = n
			}
		}
		if limit > 50 {
			limit = 50
		}

		var results []AutocompleteResult

		if handler.directoryRepo != nil && domainID != "" {
			principals, err := handler.directoryRepo.SearchPrincipals(r.Context(), directory.SearchPrincipalsRequest{
				DomainID:   domainID,
				Kinds:      []string{directory.PrincipalKindUser},
				Query:      q,
				ActiveOnly: true,
				Limit:      limit,
			})
			if err != nil {
				writeError(w, http.StatusInternalServerError, "directory search failed")
				return
			}
			for _, p := range principals {
				results = append(results, AutocompleteResult{
					Type:        "user",
					ID:          p.ID,
					DisplayName: p.DisplayName,
					Email:       p.PrimaryEmail,
				})
			}
		}

		// Search groups/distribution lists
		if handler.directoryRepo != nil && domainID != "" && limit-len(results) > 0 {
			groups, err := handler.directoryRepo.SearchPrincipals(r.Context(), directory.SearchPrincipalsRequest{
				DomainID:   domainID,
				Kinds:      []string{directory.PrincipalKindGroup, directory.PrincipalKindOrganization},
				Query:      q,
				ActiveOnly: true,
				Limit:      limit - len(results),
			})
			if err == nil {
				seen := make(map[string]bool, len(results))
				for _, res := range results {
					if res.Email != "" {
						seen[strings.ToLower(res.Email)] = true
					}
				}
				for _, p := range groups {
					if p.PrimaryEmail != "" && !seen[strings.ToLower(p.PrimaryEmail)] {
						results = append(results, AutocompleteResult{
							Type:        "group",
							ID:          p.ID,
							DisplayName: p.DisplayName,
							Email:       p.PrimaryEmail,
						})
					}
				}
			}
		}

		if handler.repo != nil && (limit-len(results)) > 0 {
			contacts, err := handler.repo.SearchContacts(r.Context(), carddavgw.SearchContactsRequest{
				UserID: userID,
				Query:  q,
				Limit:  limit - len(results),
			})
			if err != nil {
				writeError(w, http.StatusInternalServerError, "contact search failed")
				return
			}
			seen := make(map[string]bool, len(results))
			for _, res := range results {
				if res.Email != "" {
					seen[strings.ToLower(res.Email)] = true
				}
			}
			for _, c := range contacts {
				fn := vcardPropValue(c.VCard, "FN")
				email := vcardPropValue(c.VCard, "EMAIL")
				if email != "" && seen[strings.ToLower(email)] {
					continue
				}
				results = append(results, AutocompleteResult{
					Type:          "contact",
					ID:            c.ID,
					DisplayName:   fn,
					Email:         email,
					AddressBookID: c.AddressBookID,
				})
			}
		}

		writeJSON(w, http.StatusOK, AutocompleteResponse{Results: results})
	})

	mux.HandleFunc("GET /api/mail/directory/users", func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r, "q", "limit", "user_id") {
			return
		}
		if handler.directoryRepo == nil {
			writeError(w, http.StatusServiceUnavailable, "directory not available")
			return
		}
		companyID, domainID, ok := directoryScopeFromRequest(w, r, handler, tokenManager)
		if !ok {
			return
		}
		q := r.URL.Query().Get("q")
		limit := 50
		if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
			if n, ok := parsePositiveInt(limitStr); ok {
				limit = n
			}
		}
		if limit > 200 {
			limit = 200
		}
		principals, err := handler.directoryRepo.SearchPrincipals(r.Context(), directory.SearchPrincipalsRequest{
			CompanyID:  companyID,
			DomainID:   domainID,
			Kinds:      []string{directory.PrincipalKindUser},
			Query:      q,
			ActiveOnly: true,
			Limit:      limit,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "directory search failed")
			return
		}
		type DirectoryUser struct {
			ID          string `json:"id"`
			DisplayName string `json:"display_name"`
			Email       string `json:"email"`
			AvatarURL   string `json:"avatar_url,omitempty"`
		}
		users := make([]DirectoryUser, 0, len(principals))
		for _, p := range principals {
			users = append(users, DirectoryUser{
				ID:          p.ID,
				DisplayName: p.DisplayName,
				Email:       p.PrimaryEmail,
				AvatarURL:   p.AvatarURL,
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{"users": users})
	})

	// GET /api/mail/directory/org-tree — org units with their members
	mux.HandleFunc("GET /api/mail/directory/org-tree", func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r, "user_id") {
			return
		}
		if handler.directoryRepo == nil {
			writeError(w, http.StatusServiceUnavailable, "directory not available")
			return
		}
		companyID, domainID, ok := directoryScopeFromRequest(w, r, handler, tokenManager)
		if !ok {
			return
		}
		orgs, err := handler.directoryRepo.ListOrgTree(r.Context(), companyID, domainID)
		if err != nil {
			slog.ErrorContext(r.Context(), "list org tree failed", "error", err, "company_id", companyID, "domain_id", domainID)
			writeError(w, http.StatusInternalServerError, "org search failed")
			return
		}
		type OrgMember struct {
			ID          string `json:"id"`
			DisplayName string `json:"display_name"`
			Email       string `json:"email"`
			AvatarURL   string `json:"avatar_url,omitempty"`
		}
		type OrgUnit struct {
			ID          string      `json:"id"`
			DisplayName string      `json:"display_name"`
			ParentID    string      `json:"parent_id,omitempty"`
			Depth       int         `json:"depth"`
			Members     []OrgMember `json:"members"`
		}
		orgIDs := make([]string, 0, len(orgs))
		for _, org := range orgs {
			orgIDs = append(orgIDs, org.ID)
		}
		membersByOrgID, err := handler.directoryRepo.ListOrgMembersByOrgIDs(r.Context(), companyID, domainID, orgIDs, 100)
		if err != nil {
			membersByOrgID = map[string][]directory.Principal{}
		}
		units := make([]OrgUnit, 0, len(orgs))
		for _, org := range orgs {
			members := membersByOrgID[org.ID]
			mList := make([]OrgMember, 0, len(members))
			for _, m := range members {
				mList = append(mList, OrgMember{ID: m.ID, DisplayName: m.DisplayName, Email: m.PrimaryEmail, AvatarURL: m.AvatarURL})
			}
			units = append(units, OrgUnit{
				ID: org.ID, DisplayName: org.DisplayName,
				ParentID: org.ParentID, Depth: org.Depth,
				Members: mList,
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{"units": units})
	})

	// GET /api/mail/directory/profile?email={email} — user profile with org unit + title
	mux.HandleFunc("GET /api/mail/directory/profile", func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r, "email", "user_id") {
			return
		}
		if handler.directoryRepo == nil {
			writeError(w, http.StatusServiceUnavailable, "directory not available")
			return
		}

		_, _, ok := directoryScopeFromRequest(w, r, handler, tokenManager)
		if !ok {
			return
		}

		email := strings.TrimSpace(r.URL.Query().Get("email"))
		if email == "" {
			writeError(w, http.StatusBadRequest, "email is required")
			return
		}

		principal, err := handler.directoryRepo.ResolveUserByEmail(r.Context(), directory.ResolveUserByEmailRequest{
			Email:      email,
			ActiveOnly: true,
		})
		if err != nil {
			// User not found — return empty profile (not an error; caller decides)
			writeJSON(w, http.StatusOK, map[string]any{"found": false})
			return
		}

		type ProfileResponse struct {
			Found       bool   `json:"found"`
			DisplayName string `json:"display_name,omitempty"`
			OrgUnitName string `json:"org_unit_name,omitempty"`
			Title       string `json:"title,omitempty"`
		}

		resp := ProfileResponse{Found: true, DisplayName: principal.DisplayName}

		if handler.orgProfiler != nil {
			if memberships, err := handler.orgProfiler.GetMembershipsForUser(r.Context(), principal.ID); err == nil && len(memberships) > 0 {
				// Primary membership (already sorted: is_primary DESC) takes priority.
				resp.OrgUnitName = memberships[0].UnitName
				resp.Title = memberships[0].Title
			}
		}

		writeJSON(w, http.StatusOK, resp)
	})
}

func directoryScopeFromRequest(w http.ResponseWriter, r *http.Request, handler *ContactHandler, tokenManager *auth.TokenManager) (string, string, bool) {
	if info, ok := apikeys.KeyInfoFromContext(r.Context()); ok && info != nil && strings.TrimSpace(info.UserID) != "" {
		userID, ok := userScopedAPIKeyUserIDFromRequest(w, r, info, "", "")
		if !ok {
			return "", "", false
		}
		principal, err := handler.directoryRepo.ResolvePrincipal(r.Context(), directory.ResolvePrincipalRequest{
			ID:   userID,
			Kind: directory.PrincipalKindUser,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "directory lookup failed")
			return "", "", false
		}
		return principal.CompanyID, principal.DomainID, true
	}
	if tokenManager != nil {
		claims, ok := claimsFromRequest(w, r, tokenManager)
		if !ok {
			return "", "", false
		}
		return claims.CompanyID, claims.DomainID, true
	}
	userID, ok := userIDFromRequest(w, r, nil)
	if !ok {
		return "", "", false
	}
	principal, err := handler.directoryRepo.ResolvePrincipal(r.Context(), directory.ResolvePrincipalRequest{
		ID:   userID,
		Kind: directory.PrincipalKindUser,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "directory lookup failed")
		return "", "", false
	}
	return principal.CompanyID, principal.DomainID, true
}

// vcardPropValue returns the first value of the named vCard property,
// handling RFC 6350 CRLF/LF line endings and line unfolding.
func vcardPropValue(vcard []byte, prop string) string {
	// Normalise line endings to LF, then unfold continuation lines.
	text := strings.ReplaceAll(string(vcard), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = strings.ReplaceAll(text, "\n ", "")
	text = strings.ReplaceAll(text, "\n\t", "")

	prop = strings.ToUpper(prop)
	for _, line := range strings.Split(text, "\n") {
		colon := strings.IndexByte(line, ':')
		if colon < 0 {
			continue
		}
		namePart := line[:colon]
		if semi := strings.IndexByte(namePart, ';'); semi >= 0 {
			namePart = namePart[:semi]
		}
		if strings.ToUpper(strings.TrimSpace(namePart)) == prop {
			return strings.TrimSpace(line[colon+1:])
		}
	}
	return ""
}

func parsePositiveInt(s string) (int, bool) {
	if s == "" {
		return 0, false
	}
	var n int
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int(c-'0')
	}
	return n, n > 0
}
