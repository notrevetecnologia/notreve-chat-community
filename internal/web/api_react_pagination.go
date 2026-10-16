package web

import (
	"errors"
	"net/http"
	"strings"
)

type reactPageRequest struct {
	Limit  int
	Offset int
}

func parseReactPage(r *http.Request) (reactPageRequest, error) {
	limit, err := parseReactLimit(r.URL.Query().Get("limit"))
	if err != nil {
		return reactPageRequest{}, err
	}
	offset, err := decodeReactCursor(r.URL.Query().Get("cursor"))
	if err != nil {
		return reactPageRequest{}, err
	}
	return reactPageRequest{Limit: limit, Offset: offset}, nil
}

func reactPageMeta(total, limit, offset int, filters map[string]string) map[string]interface{} {
	end := offset + limit
	if end > total {
		end = total
	}
	hasMore := end < total
	nextCursor := ""
	if hasMore {
		nextCursor = encodeReactCursor(end)
	}
	meta := map[string]interface{}{
		"limit": limit, "total": total, "has_more": hasMore,
		"next_cursor": nextCursor, "offset": offset,
	}
	if len(filters) > 0 {
		meta["filters"] = filters
	}
	return meta
}

func reactPageSlice[T any](items []T, limit, offset int) ([]T, error) {
	if offset < 0 {
		return nil, errors.New("cursor inválido")
	}
	if offset > len(items) {
		offset = len(items)
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	page := make([]T, end-offset)
	copy(page, items[offset:end])
	return page, nil
}

func reactContainsFold(values []string, query string) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return true
	}
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), query) {
			return true
		}
	}
	return false
}

func reactQueryFilter(values map[string]string) map[string]string {
	filter := map[string]string{}
	for key, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			filter[key] = value
		}
	}
	return filter
}
