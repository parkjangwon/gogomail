package webdavgw

import (
	"encoding/xml"
	"fmt"
	"path"
	"strings"
	"time"
)

type Resource struct {
	Href                string
	Name                string
	Size                int64
	IsCollection        bool
	Modified            time.Time
	ContentType         string
	// RFC 4331 quota properties; nil means the property is not reported.
	QuotaUsedBytes      *int64
	QuotaAvailableBytes *int64
}

type propfindRequest struct {
	XMLName xml.Name `xml:"propfind"`
	Props   []prop   `xml:"prop>"`
}

type prop struct {
	XMLName xml.Name
}

type multistatus struct {
	XMLName   xml.Name   `xml:"d:multistatus"`
	XmlnsD    string     `xml:"xmlns:d,attr"`
	Responses []response `xml:"d:response"`
}

type response struct {
	Href     string   `xml:"d:href"`
	Propstat propstat `xml:"d:propstat"`
}

type propstat struct {
	Prop   properties `xml:"d:prop"`
	Status string     `xml:"d:status"`
}

type properties struct {
	DisplayName         string   `xml:"d:displayname,omitempty"`
	ResourceType        *resType `xml:"d:resourcetype,omitempty"`
	ContentLength       int64    `xml:"d:getcontentlength,omitempty"`
	ContentType         string   `xml:"d:getcontenttype,omitempty"`
	LastModified        string   `xml:"d:getlastmodified,omitempty"`
	QuotaUsedBytes      *int64   `xml:"d:quota-used-bytes"`
	QuotaAvailableBytes *int64   `xml:"d:quota-available-bytes"`
}

type resType struct {
	Collection *struct{} `xml:"d:collection,omitempty"`
}

func MarshalPropfindResponse(resources []Resource) ([]byte, error) {
	ms := multistatus{
		XmlnsD: "DAV:",
	}

	for _, r := range resources {
		props := properties{
			DisplayName:         r.Name,
			ContentLength:       r.Size,
			ContentType:         r.ContentType,
			LastModified:        r.Modified.UTC().Format(httpTimeFormat),
			QuotaUsedBytes:      r.QuotaUsedBytes,
			QuotaAvailableBytes: r.QuotaAvailableBytes,
		}
		if r.IsCollection {
			props.ResourceType = &resType{Collection: &struct{}{}}
		}

		ms.Responses = append(ms.Responses, response{
			Href: r.Href,
			Propstat: propstat{
				Prop:   props,
				Status: "HTTP/1.1 200 OK",
			},
		})
	}

	xmlData := []byte(xml.Header)
	output, err := xml.MarshalIndent(ms, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal propfind: %w", err)
	}
	return append(xmlData, output...), nil
}

func ParsePropfindRequest(data []byte) ([]string, error) {
	var pf struct {
		XMLName xml.Name `xml:"propfind"`
		Prop    struct {
			Fields []xmlField `xml:",any"`
		} `xml:"prop"`
	}

	if err := xml.Unmarshal(data, &pf); err != nil {
		return nil, fmt.Errorf("unmarshal propfind: %w", err)
	}

	var props []string
	for _, f := range pf.Prop.Fields {
		props = append(props, f.XMLName.Local)
	}
	return props, nil
}

type xmlField struct {
	XMLName xml.Name
}

func NormalizeHref(href string) string {
	trailing := strings.HasSuffix(href, "/")
	href = path.Clean("/" + strings.TrimPrefix(href, "/"))
	if href == "" {
		return "/"
	}
	if trailing && !strings.HasSuffix(href, "/") {
		href += "/"
	}
	return href
}

const httpTimeFormat = "Mon, 02 Jan 2006 15:04:05 GMT"
