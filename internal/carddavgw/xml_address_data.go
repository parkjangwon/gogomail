package carddavgw

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

type UnsupportedAddressDataError struct {
	Attribute string
	Value     string
}

func (e UnsupportedAddressDataError) Error() string {
	return fmt.Sprintf("unsupported CardDAV address-data %s %q", e.Attribute, e.Value)
}

type UnsupportedCollationError struct {
	Value string
}

func (e UnsupportedCollationError) Error() string {
	return fmt.Sprintf("unsupported CardDAV text-match collation %q", e.Value)
}

type UnsupportedFilterElementError struct {
	Name XMLName
}

func (e UnsupportedFilterElementError) Error() string {
	return fmt.Sprintf("unsupported CardDAV filter element {%s}%s", e.Name.Space, e.Name.Local)
}

func parseAddressDataElement(dec *xml.Decoder, el xml.StartElement) ([]string, error) {
	if err := validateAddressDataAttributes(el); err != nil {
		return nil, err
	}
	var properties []string
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return nil, fmt.Errorf("unterminated address-data element")
		}
		if err != nil {
			return nil, fmt.Errorf("decode address-data element: %w", err)
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			if !sameXMLName(tok.Name, CardDAVNamespace, "prop") {
				if err := skipElement(dec, tok.Name); err != nil {
					return nil, err
				}
				continue
			}
			propName, err := filterNameAttribute(tok, "address-data prop")
			if err != nil {
				return nil, err
			}
			if len(properties) >= MaxWebDAVProperties {
				return nil, fmt.Errorf("too many CardDAV address-data properties")
			}
			properties = append(properties, propName)
			if err := skipElement(dec, tok.Name); err != nil {
				return nil, err
			}
		case xml.EndElement:
			if sameName(tok.Name, el.Name) {
				return properties, nil
			}
		}
	}
}

func validateAddressDataAttributes(el xml.StartElement) error {
	for _, attr := range el.Attr {
		if strings.ContainsAny(attr.Value, "\r\n") {
			return fmt.Errorf("CardDAV address-data attribute is invalid")
		}
		switch attr.Name.Local {
		case "content-type":
			contentType := strings.ToLower(strings.TrimSpace(attr.Value))
			if contentType != "text/vcard" {
				return UnsupportedAddressDataError{Attribute: "content-type", Value: contentType}
			}
		case "version":
			version := strings.TrimSpace(attr.Value)
			if version != "3.0" && version != "4.0" {
				return UnsupportedAddressDataError{Attribute: "version", Value: version}
			}
		}
	}
	return nil
}
