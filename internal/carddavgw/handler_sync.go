package carddavgw

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

func (h *Handler) syncCollectionReport(ctx context.Context, userID string, resource ResourcePath, report ReportRequest, currentUserPrivileges []XMLName) ([]MultiStatusResponse, string, error) {
	if resource.Kind != ResourceAddressBookCollection {
		return nil, "", fmt.Errorf("sync-collection requires an address-book collection resource")
	}
	book, err := h.Store.LookupAddressBook(ctx, userID, resource.AddressBookID)
	if err != nil {
		if report.SyncToken == "" {
			return nil, "", err
		}
		responses, syncToken, changeErr := h.syncChangeResponses(ctx, userID, resource, report, currentUserPrivileges)
		if changeErr != nil {
			return nil, "", changeErr
		}
		return responses, syncToken, nil
	}
	if report.SyncToken != "" {
		if report.SyncToken != book.SyncToken {
			responses, syncToken, err := h.syncChangeResponses(ctx, userID, resource, report, currentUserPrivileges)
			if err != nil {
				return nil, "", err
			}
			return responses, syncToken, nil
		}
		return nil, book.SyncToken, nil
	}
	limit := report.Limit
	if limit <= 0 {
		limit = MaxWebDAVReportLimit
	}
	objects, err := h.listAddressBookObjectsBounded(ctx, userID, resource.AddressBookID, limit+1)
	if err != nil {
		return nil, "", err
	}
	if len(objects) > limit {
		return nil, "", TruncatedResultsError{Operation: "sync-collection limit"}
	}
	propfind := PropfindRequest{Kind: PropfindProp, Properties: report.Properties}
	responses := make([]MultiStatusResponse, 0, len(objects))
	for _, object := range objects {
		props, err := ContactObjectProperties(userID, object)
		if err != nil {
			return nil, "", err
		}
		props = withCurrentUserPrivileges(props, ResourceContactObject, currentUserPrivileges)
		if containsXMLName(report.Properties, PropAddressData) {
			dataProp, err := ContactObjectDataPropertyWithProperties(object.VCard, report.AddressDataProperties)
			if err != nil {
				return nil, "", err
			}
			props = append(props, dataProp)
		}
		href, err := ContactObjectPath(userID, object.AddressBookID, object.ObjectName)
		if err != nil {
			return nil, "", err
		}
		responses = append(responses, responseForProperties(href, propfind, props))
	}
	return responses, book.SyncToken, nil
}

func (h *Handler) listAddressBookObjectsBounded(ctx context.Context, userID string, addressBookID string, limit int) ([]ContactObject, error) {
	if limiter, ok := h.Store.(AddressBookObjectLimiter); ok {
		return limiter.ListAddressBookObjectsLimit(ctx, userID, addressBookID, limit)
	}
	return h.Store.ListAddressBookObjects(ctx, userID, addressBookID)
}

func (h *Handler) syncChangeResponses(ctx context.Context, userID string, resource ResourcePath, report ReportRequest, currentUserPrivileges []XMLName) ([]MultiStatusResponse, string, error) {
	store, ok := h.Store.(SyncChangeStore)
	if !ok {
		return nil, "", InvalidSyncTokenError{Token: report.SyncToken}
	}
	limit := report.Limit
	if limit <= 0 {
		limit = MaxWebDAVReportLimit
	}
	fetchLimit := MaxWebDAVReportLimit + 1
	includeAddressData := containsXMLName(report.Properties, PropAddressData)
	if changeWithObjectStore, ok := store.(AddressBookChangeWithObjectStore); ok {
		changesWithObject, err := changeWithObjectStore.ListAddressBookChangesWithObjectsSince(ctx, ListAddressBookChangesSinceRequest{
			UserID:        userID,
			AddressBookID: resource.AddressBookID,
			SyncToken:     report.SyncToken,
			Limit:         fetchLimit,
		}, false)
		if err != nil {
			return nil, "", err
		}
		if len(changesWithObject) > MaxWebDAVReportLimit {
			return nil, "", TruncatedResultsError{Operation: "sync-collection limit"}
		}
		syncToken := report.SyncToken
		for _, item := range changesWithObject {
			if strings.TrimSpace(item.Change.SyncToken) != "" {
				syncToken = strings.TrimSpace(item.Change.SyncToken)
			}
		}
		changesWithObject = coalesceAddressBookChangesWithObjects(changesWithObject)
		if len(changesWithObject) > limit {
			return nil, "", TruncatedResultsError{Operation: "sync-collection limit"}
		}
		objectsWithData := map[contactObjectLookupKey]ContactObject{}
		if includeAddressData {
			requestedByAddressBook := make(map[string][]string)
			for _, item := range changesWithObject {
				change := item.Change
				if change.Action == "contact-deleted" || !item.HasObject {
					continue
				}
				requestedByAddressBook[change.AddressBookID] = append(requestedByAddressBook[change.AddressBookID], change.ObjectName)
			}
			objectsWithData, err = h.lookupContactObjectsByNames(ctx, userID, requestedByAddressBook)
			if err != nil {
				return nil, "", err
			}
		}
		propfind := PropfindRequest{Kind: PropfindProp, Properties: report.Properties}
		responses := make([]MultiStatusResponse, 0, len(changesWithObject))
		for _, item := range changesWithObject {
			change := item.Change
			href, err := ContactObjectPath(userID, change.AddressBookID, change.ObjectName)
			if err != nil {
				return nil, "", err
			}
			if change.Action == "contact-deleted" || !item.HasObject {
				responses = append(responses, MultiStatusResponse{Href: href, Status: http.StatusNotFound})
				continue
			}
			object := item.Object
			if includeAddressData {
				objectWithData, ok := objectsWithData[contactObjectLookupKey{addressBookID: change.AddressBookID, objectName: change.ObjectName}]
				if !ok {
					responses = append(responses, MultiStatusResponse{Href: href, Status: http.StatusNotFound})
					continue
				}
				object = objectWithData
			}
			props, err := ContactObjectProperties(userID, object)
			if err != nil {
				return nil, "", err
			}
			props = withCurrentUserPrivileges(props, ResourceContactObject, currentUserPrivileges)
			if includeAddressData {
				dataProp, err := ContactObjectDataPropertyWithProperties(object.VCard, report.AddressDataProperties)
				if err != nil {
					return nil, "", err
				}
				props = append(props, dataProp)
			}
			responses = append(responses, responseForProperties(href, propfind, props))
		}
		return responses, syncToken, nil
	}
	changes, err := store.ListAddressBookChangesSince(ctx, ListAddressBookChangesSinceRequest{
		UserID:        userID,
		AddressBookID: resource.AddressBookID,
		SyncToken:     report.SyncToken,
		Limit:         fetchLimit,
	})
	if err != nil {
		return nil, "", err
	}
	if len(changes) > MaxWebDAVReportLimit {
		return nil, "", TruncatedResultsError{Operation: "sync-collection limit"}
	}
	syncToken := report.SyncToken
	for _, change := range changes {
		if strings.TrimSpace(change.SyncToken) != "" {
			syncToken = strings.TrimSpace(change.SyncToken)
		}
	}
	changes = coalesceAddressBookChanges(changes)
	if len(changes) > limit {
		return nil, "", TruncatedResultsError{Operation: "sync-collection limit"}
	}
	propfind := PropfindRequest{Kind: PropfindProp, Properties: report.Properties}
	responses := make([]MultiStatusResponse, 0, len(changes))
	for _, change := range changes {
		href, err := ContactObjectPath(userID, change.AddressBookID, change.ObjectName)
		if err != nil {
			return nil, "", err
		}
		if change.Action == "contact-deleted" {
			responses = append(responses, MultiStatusResponse{Href: href, Status: http.StatusNotFound})
			continue
		}
		object, err := h.Store.LookupContactObject(ctx, userID, change.AddressBookID, change.ObjectName)
		if err != nil {
			responses = append(responses, MultiStatusResponse{Href: href, Status: http.StatusNotFound})
			continue
		}
		props, err := ContactObjectProperties(userID, object)
		if err != nil {
			return nil, "", err
		}
		props = withCurrentUserPrivileges(props, ResourceContactObject, currentUserPrivileges)
		if containsXMLName(report.Properties, PropAddressData) {
			dataProp, err := ContactObjectDataPropertyWithProperties(object.VCard, report.AddressDataProperties)
			if err != nil {
				return nil, "", err
			}
			props = append(props, dataProp)
		}
		responses = append(responses, responseForProperties(href, propfind, props))
	}
	return responses, syncToken, nil
}

type coalescedAddressBookChangeWithObject struct {
	item   AddressBookChangeWithObject
	active bool
}

func coalesceAddressBookChangesWithObjects(changes []AddressBookChangeWithObject) []AddressBookChangeWithObject {
	entries := make([]coalescedAddressBookChangeWithObject, 0, len(changes))
	latestIndex := make(map[contactObjectLookupKey]int, len(changes))
	for _, item := range changes {
		if !addressBookChangeHasObjectResponse(item.Change) {
			continue
		}
		key := contactObjectLookupKey{addressBookID: item.Change.AddressBookID, objectName: item.Change.ObjectName}
		if previous, ok := latestIndex[key]; ok {
			entries[previous].active = false
		}
		latestIndex[key] = len(entries)
		entries = append(entries, coalescedAddressBookChangeWithObject{item: item, active: true})
	}
	coalesced := make([]AddressBookChangeWithObject, 0, len(latestIndex))
	for _, entry := range entries {
		if entry.active {
			coalesced = append(coalesced, entry.item)
		}
	}
	return coalesced
}

type coalescedAddressBookChange struct {
	change AddressBookChange
	active bool
}

func coalesceAddressBookChanges(changes []AddressBookChange) []AddressBookChange {
	entries := make([]coalescedAddressBookChange, 0, len(changes))
	latestIndex := make(map[contactObjectLookupKey]int, len(changes))
	for _, change := range changes {
		if !addressBookChangeHasObjectResponse(change) {
			continue
		}
		key := contactObjectLookupKey{addressBookID: change.AddressBookID, objectName: change.ObjectName}
		if previous, ok := latestIndex[key]; ok {
			entries[previous].active = false
		}
		latestIndex[key] = len(entries)
		entries = append(entries, coalescedAddressBookChange{change: change, active: true})
	}
	coalesced := make([]AddressBookChange, 0, len(latestIndex))
	for _, entry := range entries {
		if entry.active {
			coalesced = append(coalesced, entry.change)
		}
	}
	return coalesced
}

func addressBookChangeHasObjectResponse(change AddressBookChange) bool {
	return change.Action != "addressbook-created" &&
		change.Action != "addressbook-updated" &&
		change.Action != "addressbook-deleted" &&
		strings.TrimSpace(change.ObjectName) != ""
}
