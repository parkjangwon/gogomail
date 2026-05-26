package mailservice

import (
	"fmt"
	"strings"
)

const maxServiceResourceIDBytes = 200

func validateServiceResourceID(field string, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("%s is required", field)
	}
	if strings.ContainsAny(id, "\r\n") {
		return fmt.Errorf("%s must not contain CR or LF", field)
	}
	if len(id) > maxServiceResourceIDBytes {
		return fmt.Errorf("%s is too long", field)
	}
	return nil
}

func normalizeStringList(values []string) []string {
	for i := range values {
		values[i] = strings.TrimSpace(values[i])
	}
	return values
}
