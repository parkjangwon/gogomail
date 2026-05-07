package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gogomail/gogomail/internal/auth"
	"github.com/gogomail/gogomail/internal/carddavgw"
)

type ContactRepo interface {
	ListAddressBooks(ctx context.Context, req carddavgw.ListAddressBooksRequest) ([]carddavgw.AddressBook, error)
	CreateAddressBook(ctx context.Context, req carddavgw.CreateAddressBookRequest) (carddavgw.AddressBook, error)
	GetAddressBook(ctx context.Context, req carddavgw.GetAddressBookRequest) (carddavgw.AddressBook, error)
	UpdateAddressBook(ctx context.Context, req carddavgw.UpdateAddressBookRequest) (carddavgw.AddressBook, error)
	DeleteAddressBook(ctx context.Context, req carddavgw.DeleteAddressBookRequest) (carddavgw.AddressBook, error)
	ListContactObjects(ctx context.Context, req carddavgw.ListContactObjectsRequest) ([]carddavgw.ContactObject, error)
	GetContactObject(ctx context.Context, req carddavgw.GetContactObjectRequest) (carddavgw.ContactObject, error)
	UpsertContactObject(ctx context.Context, req carddavgw.UpsertContactObjectRequest) (carddavgw.ContactObject, error)
	DeleteContactObject(ctx context.Context, req carddavgw.DeleteContactObjectRequest) (carddavgw.ContactObject, error)
}

type ContactHandler struct {
	repo ContactRepo
}

func NewContactHandler(repo ContactRepo) *ContactHandler {
	return &ContactHandler{repo: repo}
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

func RegisterContactRoutes(mux *http.ServeMux, handler *ContactHandler, tokenManager *auth.TokenManager) {
	allows := []string{}
	if tokenManager == nil {
		allows = []string{"user_id"}
	}

	mux.HandleFunc("GET /api/v1/addressbooks", func(w http.ResponseWriter, r *http.Request) {
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

	mux.HandleFunc("POST /api/v1/addressbooks", func(w http.ResponseWriter, r *http.Request) {
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
		w.Header().Set("Location", fmt.Sprintf("/api/v1/addressbooks/%s", addressBook.ID))
		writeJSON(w, http.StatusCreated, AddressBookEnvelope{AddressBook: addressBook})
	})

	mux.HandleFunc("GET /api/v1/addressbooks/{id}", func(w http.ResponseWriter, r *http.Request) {
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

	mux.HandleFunc("PATCH /api/v1/addressbooks/{id}", func(w http.ResponseWriter, r *http.Request) {
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
		addressBook, err := handler.repo.UpdateAddressBook(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("update address book: %w", err).Error())
			return
		}
		writeJSON(w, http.StatusOK, AddressBookEnvelope{AddressBook: addressBook})
	})

	mux.HandleFunc("DELETE /api/v1/addressbooks/{id}", func(w http.ResponseWriter, r *http.Request) {
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

	mux.HandleFunc("GET /api/v1/addressbooks/{id}/contacts", func(w http.ResponseWriter, r *http.Request) {
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

	mux.HandleFunc("GET /api/v1/addressbooks/{id}/contacts/{name}", func(w http.ResponseWriter, r *http.Request) {
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

	mux.HandleFunc("PUT /api/v1/addressbooks/{id}/contacts/{name}", func(w http.ResponseWriter, r *http.Request) {
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
			VCard:        body,
			ObservedETag: etag,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("upsert contact: %w", err).Error())
			return
		}
		w.Header().Set("ETag", fmt.Sprintf(`"%s"`, object.ETag))
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusOK, ContactObjectEnvelope{Contact: object})
	})

	mux.HandleFunc("DELETE /api/v1/addressbooks/{id}/contacts/{name}", func(w http.ResponseWriter, r *http.Request) {
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
			ObservedETag: etag,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("delete contact: %w", err).Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}