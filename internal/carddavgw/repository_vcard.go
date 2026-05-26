package carddavgw

import (
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"
)

var photoLineRegex = regexp.MustCompile(`(?i)^PHOTO(?:\;[^\:]*)?:`)

func extractPhotoFromVCard(vcard []byte) ([]byte, string, []byte, error) {
	lines := strings.Split(string(vcard), "\r\n")
	var photoLine strings.Builder
	var photoMediaType string
	var photoData []byte
	var filteredLines []string
	photoFound := false

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			if !photoFound {
				filteredLines = append(filteredLines, line)
			}
			continue
		}
		if photoLineRegex.MatchString(line) {
			photoFound = true
			photoLine.WriteString(line)
			photoLine.WriteString("\r\n")

			name, value, err := parseVCardContentLine(line)
			if err != nil {
				continue
			}
			if !strings.EqualFold(name, "PHOTO") {
				filteredLines = append(filteredLines, line)
				continue
			}

			for {
				if err != nil {
					break
				}
				parts := strings.SplitN(value, ",", 2)
				if len(parts) == 2 {
					photoMediaType = parts[0]
					if strings.HasPrefix(photoMediaType, "data:") {
						photoMediaType = strings.TrimPrefix(photoMediaType, "data:")
						decoded, err := base64.StdEncoding.DecodeString(parts[1])
						if err == nil && len(decoded) <= MaxPhotoBytes {
							photoData = decoded
							break
						}
					}
				}
				break
			}
		} else if photoFound && (strings.HasPrefix(strings.TrimSpace(line), " ") || strings.HasPrefix(strings.TrimSpace(line), "\t")) {
			photoLine.WriteString(line)
			photoLine.WriteString("\r\n")
		} else {
			photoFound = false
			filteredLines = append(filteredLines, line)
		}
	}

	cleanVCard := []byte(strings.Join(filteredLines, "\r\n"))
	return cleanVCard, photoMediaType, photoData, nil
}

func mergePhotoIntoVCard(vcard []byte, photoData []byte, photoMediaType string) []byte {
	if len(photoData) == 0 || photoMediaType == "" {
		return vcard
	}

	vCardStr := string(vcard)
	endIdx := strings.LastIndex(vCardStr, "END:VCARD")
	if endIdx == -1 {
		return vcard
	}

	encoded := base64.StdEncoding.EncodeToString(photoData)
	photoLine := fmt.Sprintf("PHOTO;ENCODING=base64;TYPE=%s:\r\n %s\r\n", photoMediaType, encoded)

	result := vCardStr[:endIdx] + photoLine + vCardStr[endIdx:]
	return []byte(result)
}

var categoriesLineRegex = regexp.MustCompile(`(?i)^CATEGORIES(?:\;[^\:]*)?:`)
var groupLineRegex = regexp.MustCompile(`(?i)^GROUP(?:\;[^\:]*)?:`)

func extractCategoriesAndGroupFromVCard(vcard []byte) ([]byte, []string, string, error) {
	lines := strings.Split(string(vcard), "\r\n")
	var categories []string
	var group string
	var filteredLines []string
	categoriesFound := false
	groupFound := false

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			if !categoriesFound && !groupFound {
				filteredLines = append(filteredLines, line)
			}
			continue
		}

		if categoriesLineRegex.MatchString(line) {
			categoriesFound = true
			name, value, err := parseVCardContentLine(line)
			if err == nil && strings.EqualFold(name, "CATEGORIES") {
				cats := strings.Split(value, ",")
				for _, cat := range cats {
					cat = strings.TrimSpace(cat)
					if cat != "" {
						categories = append(categories, cat)
					}
				}
			}
		} else if groupLineRegex.MatchString(line) {
			groupFound = true
			name, value, err := parseVCardContentLine(line)
			if err == nil && strings.EqualFold(name, "GROUP") {
				group = strings.TrimSpace(value)
			}
		} else if (categoriesFound || groupFound) && (strings.HasPrefix(strings.TrimSpace(line), " ") || strings.HasPrefix(strings.TrimSpace(line), "\t")) {
			continue
		} else {
			categoriesFound = false
			groupFound = false
			filteredLines = append(filteredLines, line)
		}
	}

	cleanVCard := []byte(strings.Join(filteredLines, "\r\n"))
	return cleanVCard, categories, group, nil
}

func mergeCategoriesAndGroupIntoVCard(vcard []byte, categories []string, group string) []byte {
	vCardStr := string(vcard)
	endIdx := strings.LastIndex(vCardStr, "END:VCARD")
	if endIdx == -1 {
		return vcard
	}

	var additions string
	if len(categories) > 0 {
		categoryStr := strings.Join(categories, ",")
		additions += fmt.Sprintf("CATEGORIES:%s\r\n", categoryStr)
	}
	if group != "" {
		additions += fmt.Sprintf("GROUP:%s\r\n", group)
	}

	if additions == "" {
		return vcard
	}

	result := vCardStr[:endIdx] + additions + vCardStr[endIdx:]
	return []byte(result)
}
