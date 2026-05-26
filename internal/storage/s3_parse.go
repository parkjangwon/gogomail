package storage

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

type s3ListObjectsResult struct {
	XMLName               xml.Name              `xml:"ListBucketResult"`
	IsTruncated           string                `xml:"IsTruncated"`
	ContinuationToken     *string               `xml:"ContinuationToken"`
	NextContinuationToken string                `xml:"NextContinuationToken"`
	Name                  *string               `xml:"Name"`
	Prefix                *string               `xml:"Prefix"`
	Delimiter             *string               `xml:"Delimiter"`
	StartAfter            *string               `xml:"StartAfter"`
	EncodingType          *string               `xml:"EncodingType"`
	KeyCount              *string               `xml:"KeyCount"`
	MaxKeys               *string               `xml:"MaxKeys"`
	Contents              []s3ListObjectContent `xml:"Contents"`
}

type s3ListObjectContent struct {
	Key          string  `xml:"Key"`
	Size         string  `xml:"Size"`
	ETag         *string `xml:"ETag"`
	LastModified *string `xml:"LastModified"`
}

type s3CopyResponse struct {
	XMLName      xml.Name
	Code         string  `xml:"Code"`
	Message      string  `xml:"Message"`
	RequestID    string  `xml:"RequestId"`
	HostID       string  `xml:"HostId"`
	ETag         string  `xml:"ETag"`
	LastModified *string `xml:"LastModified"`
}

const maxS3ListResponseBytes = 4 << 20
const maxS3CopyResponseBytes = 1 << 20
const maxS3ErrorPreviewFieldBytes = 1024

func decodeS3ListObjects(body io.Reader) (s3ListObjectsResult, error) {
	if body == nil {
		return s3ListObjectsResult{}, fmt.Errorf("list s3 objects: response body is required")
	}
	data, err := io.ReadAll(io.LimitReader(body, maxS3ListResponseBytes+1))
	if err != nil {
		return s3ListObjectsResult{}, fmt.Errorf("read s3 list response: %w", err)
	}
	if len(data) > maxS3ListResponseBytes {
		return s3ListObjectsResult{}, fmt.Errorf("list s3 objects: response body is too large")
	}
	if preview, ok := s3XMLError(data); ok {
		if preview == "" {
			return s3ListObjectsResult{}, fmt.Errorf("list s3 objects: embedded error")
		}
		return s3ListObjectsResult{}, fmt.Errorf("list s3 objects: embedded error: %s", preview)
	}
	if err := validateS3ListControlCardinality(data); err != nil {
		return s3ListObjectsResult{}, err
	}
	var result s3ListObjectsResult
	if err := xml.Unmarshal(data, &result); err != nil {
		return s3ListObjectsResult{}, fmt.Errorf("decode s3 list response: %w", err)
	}
	if !s3XMLNamespaceAllowed(result.XMLName.Space) {
		return s3ListObjectsResult{}, fmt.Errorf("list s3 objects: unexpected response namespace")
	}
	return result, nil
}

func validateS3ListControlCardinality(data []byte) error {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var rootDepth int
	var isTruncatedSeen bool
	var continuationTokenSeen bool
	var inContent bool
	var keySeen bool
	var sizeSeen bool
	var etagSeen bool
	var lastModifiedSeen bool
	var storageClassSeen bool
	var ownerSeen bool
	var checksumTypeSeen bool
	var restoreStatusSeen bool
	var simpleObjectMetadata string
	var simpleStandardMetadata string
	var structuredObjectMetadata string
	structuredObjectChildSeen := make(map[string]struct{})
	rootSimpleSeen := make(map[string]struct{})
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("decode s3 list response: %w", err)
		}
		switch token := token.(type) {
		case xml.StartElement:
			rootDepth++
			switch {
			case rootDepth == 2:
				switch token.Name.Local {
				case "IsTruncated":
					if !s3XMLNamespaceAllowed(token.Name.Space) {
						return fmt.Errorf("list s3 objects: unexpected response namespace")
					}
					if isTruncatedSeen {
						return fmt.Errorf("list s3 objects: duplicate IsTruncated value")
					}
					isTruncatedSeen = true
				case "NextContinuationToken":
					if !s3XMLNamespaceAllowed(token.Name.Space) {
						return fmt.Errorf("list s3 objects: unexpected response namespace")
					}
					if continuationTokenSeen {
						return fmt.Errorf("list s3 objects: duplicate continuation token")
					}
					continuationTokenSeen = true
				case "Contents":
					if !s3XMLNamespaceAllowed(token.Name.Space) {
						return fmt.Errorf("list s3 objects: unexpected response namespace")
					}
					inContent = true
					keySeen = false
					sizeSeen = false
					etagSeen = false
					lastModifiedSeen = false
					storageClassSeen = false
					ownerSeen = false
					checksumTypeSeen = false
					restoreStatusSeen = false
					simpleObjectMetadata = ""
					simpleStandardMetadata = ""
				case "CommonPrefixes":
					if !s3XMLNamespaceAllowed(token.Name.Space) {
						return fmt.Errorf("list s3 objects: unexpected response namespace")
					}
					return fmt.Errorf("list s3 objects: CommonPrefixes are unsupported")
				default:
					if s3ListStandardRootMetadata(token.Name.Local) && !s3XMLNamespaceAllowed(token.Name.Space) {
						return fmt.Errorf("list s3 objects: unexpected response namespace")
					}
					if s3ListStandardSimpleRootMetadata(token.Name.Local) {
						if _, ok := rootSimpleSeen[token.Name.Local]; ok {
							return fmt.Errorf("list s3 objects: duplicate %s value", token.Name.Local)
						}
						rootSimpleSeen[token.Name.Local] = struct{}{}
						simpleStandardMetadata = token.Name.Local
					}
				}
			case inContent && rootDepth == 3:
				switch token.Name.Local {
				case "Key":
					if !s3XMLNamespaceAllowed(token.Name.Space) {
						return fmt.Errorf("list s3 objects: unexpected response namespace")
					}
					if keySeen {
						return fmt.Errorf("list s3 objects: duplicate object key")
					}
					keySeen = true
					simpleObjectMetadata = token.Name.Local
				case "Size":
					if !s3XMLNamespaceAllowed(token.Name.Space) {
						return fmt.Errorf("list s3 objects: unexpected response namespace")
					}
					if sizeSeen {
						return fmt.Errorf("list s3 objects: duplicate object size")
					}
					sizeSeen = true
					simpleObjectMetadata = token.Name.Local
				case "ETag":
					if !s3XMLNamespaceAllowed(token.Name.Space) {
						return fmt.Errorf("list s3 objects: unexpected response namespace")
					}
					if etagSeen {
						return fmt.Errorf("list s3 objects: duplicate object etag")
					}
					etagSeen = true
					simpleObjectMetadata = token.Name.Local
				case "LastModified":
					if !s3XMLNamespaceAllowed(token.Name.Space) {
						return fmt.Errorf("list s3 objects: unexpected response namespace")
					}
					if lastModifiedSeen {
						return fmt.Errorf("list s3 objects: duplicate object last-modified")
					}
					lastModifiedSeen = true
					simpleObjectMetadata = token.Name.Local
				default:
					if s3ListStandardObjectMetadata(token.Name.Local) && !s3XMLNamespaceAllowed(token.Name.Space) {
						return fmt.Errorf("list s3 objects: unexpected response namespace")
					}
					switch token.Name.Local {
					case "StorageClass":
						if storageClassSeen {
							return fmt.Errorf("list s3 objects: duplicate object StorageClass value")
						}
						storageClassSeen = true
					case "Owner":
						if ownerSeen {
							return fmt.Errorf("list s3 objects: duplicate object Owner value")
						}
						ownerSeen = true
						structuredObjectMetadata = token.Name.Local
						clear(structuredObjectChildSeen)
					case "ChecksumType":
						if checksumTypeSeen {
							return fmt.Errorf("list s3 objects: duplicate object ChecksumType value")
						}
						checksumTypeSeen = true
					case "RestoreStatus":
						if restoreStatusSeen {
							return fmt.Errorf("list s3 objects: duplicate object RestoreStatus value")
						}
						restoreStatusSeen = true
						structuredObjectMetadata = token.Name.Local
						clear(structuredObjectChildSeen)
					}
					if s3ListStandardSimpleObjectMetadata(token.Name.Local) {
						simpleStandardMetadata = token.Name.Local
					}
				}
			case inContent && rootDepth > 3 && structuredObjectMetadata != "":
				if !s3XMLNamespaceAllowed(token.Name.Space) {
					return fmt.Errorf("list s3 objects: object %s metadata contains unexpected namespace", structuredObjectMetadata)
				}
				if rootDepth > 4 {
					return fmt.Errorf("list s3 objects: object %s metadata contains nested element %q", structuredObjectMetadata, token.Name.Local)
				}
				if !s3ListStructuredObjectMetadataChildAllowed(structuredObjectMetadata, token.Name.Local) {
					return fmt.Errorf("list s3 objects: object %s metadata contains unsupported child %q", structuredObjectMetadata, token.Name.Local)
				}
				if _, ok := structuredObjectChildSeen[token.Name.Local]; ok {
					return fmt.Errorf("list s3 objects: object %s metadata contains duplicate child %q", structuredObjectMetadata, token.Name.Local)
				}
				structuredObjectChildSeen[token.Name.Local] = struct{}{}
			case rootDepth > 2 && simpleStandardMetadata != "":
				return fmt.Errorf("list s3 objects: metadata %s contains nested element %q", simpleStandardMetadata, token.Name.Local)
			case inContent && rootDepth > 3 && simpleObjectMetadata != "":
				return fmt.Errorf("list s3 objects: object %s metadata contains nested element %q", simpleObjectMetadata, token.Name.Local)
			}
		case xml.EndElement:
			if simpleStandardMetadata == token.Name.Local {
				simpleStandardMetadata = ""
			}
			if inContent && rootDepth == 3 && simpleObjectMetadata == token.Name.Local {
				simpleObjectMetadata = ""
			}
			if inContent && rootDepth == 3 && structuredObjectMetadata == token.Name.Local {
				structuredObjectMetadata = ""
				clear(structuredObjectChildSeen)
			}
			if inContent && rootDepth == 2 && token.Name.Local == "Contents" {
				inContent = false
				simpleObjectMetadata = ""
				simpleStandardMetadata = ""
				structuredObjectMetadata = ""
				clear(structuredObjectChildSeen)
			}
			if rootDepth > 0 {
				rootDepth--
			}
		case xml.CharData:
			if inContent && rootDepth == 3 && structuredObjectMetadata != "" && len(bytes.TrimSpace(token)) > 0 {
				return fmt.Errorf("list s3 objects: object %s metadata contains direct text", structuredObjectMetadata)
			}
		}
	}
}

func s3ListStandardSimpleRootMetadata(local string) bool {
	switch local {
	case "Name", "Prefix", "Delimiter", "MaxKeys", "KeyCount", "ContinuationToken", "StartAfter", "EncodingType":
		return true
	default:
		return false
	}
}

func s3ListStandardRootMetadata(local string) bool {
	switch local {
	case "Name", "Prefix", "Delimiter", "MaxKeys", "KeyCount", "ContinuationToken", "StartAfter", "EncodingType":
		return true
	default:
		return false
	}
}

func s3ListStandardSimpleObjectMetadata(local string) bool {
	switch local {
	case "StorageClass", "ChecksumAlgorithm", "ChecksumType":
		return true
	default:
		return false
	}
}

func s3ListStandardObjectMetadata(local string) bool {
	switch local {
	case "StorageClass", "Owner", "ChecksumAlgorithm", "ChecksumType", "RestoreStatus":
		return true
	default:
		return false
	}
}

func s3ListStructuredObjectMetadataChildAllowed(parent string, local string) bool {
	switch parent {
	case "Owner":
		switch local {
		case "ID", "DisplayName":
			return true
		default:
			return false
		}
	case "RestoreStatus":
		switch local {
		case "IsRestoreInProgress", "RestoreExpiryDate":
			return true
		default:
			return false
		}
	default:
		return false
	}
}

func validateS3CopyResponse(body io.Reader) error {
	if body == nil {
		return fmt.Errorf("copy s3 object: response body is required")
	}
	data, err := io.ReadAll(io.LimitReader(body, maxS3CopyResponseBytes+1))
	if err != nil {
		return fmt.Errorf("read s3 copy response: %w", err)
	}
	if len(data) > maxS3CopyResponseBytes {
		return fmt.Errorf("copy s3 object: response body is too large")
	}
	if strings.TrimSpace(string(data)) == "" {
		return fmt.Errorf("copy s3 object: response body is required")
	}
	if err := validateS3CopyResultShape(data); err != nil {
		return err
	}
	var response s3CopyResponse
	if err := xml.Unmarshal(data, &response); err != nil {
		return fmt.Errorf("decode s3 copy response: %w", err)
	}
	switch response.XMLName.Local {
	case "CopyObjectResult":
		if !s3XMLNamespaceAllowed(response.XMLName.Space) {
			return fmt.Errorf("copy s3 object: unexpected response namespace")
		}
		if strings.TrimSpace(response.ETag) == "" {
			return fmt.Errorf("copy s3 object: etag is required")
		}
		if strings.TrimSpace(response.ETag) != response.ETag {
			return fmt.Errorf("copy s3 object: invalid etag")
		}
		if cleanS3ETag(response.ETag) == "" {
			return fmt.Errorf("copy s3 object: invalid etag")
		}
		if response.LastModified != nil {
			if strings.TrimSpace(*response.LastModified) == "" {
				return fmt.Errorf("copy s3 object: last-modified is empty")
			}
			if _, ok := parseS3ListTime(*response.LastModified); !ok {
				return fmt.Errorf("copy s3 object: invalid last-modified")
			}
		}
		return nil
	case "Error":
		preview, _ := s3XMLError(data)
		if preview == "" {
			return fmt.Errorf("copy s3 object: embedded error")
		}
		return fmt.Errorf("copy s3 object: embedded error: %s", preview)
	default:
		return fmt.Errorf("copy s3 object: unexpected response %q", response.XMLName.Local)
	}
}

func validateS3CopyResultShape(data []byte) error {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var rootName xml.Name
	var rootDepth int
	var etagSeen bool
	var lastModifiedSeen bool
	var simpleCopyMetadata string
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("decode s3 copy response: %w", err)
		}
		switch token := token.(type) {
		case xml.StartElement:
			rootDepth++
			if rootDepth == 1 {
				rootName = token.Name
				continue
			}
			if rootDepth > 2 && rootName.Local == "CopyObjectResult" && simpleCopyMetadata != "" {
				return fmt.Errorf("copy s3 object: %s metadata contains nested element %q", simpleCopyMetadata, token.Name.Local)
			}
			if rootDepth != 2 || rootName.Local != "CopyObjectResult" {
				continue
			}
			switch token.Name.Local {
			case "ETag":
				if !s3XMLNamespaceAllowed(token.Name.Space) {
					return fmt.Errorf("copy s3 object: unexpected response namespace")
				}
				if etagSeen {
					return fmt.Errorf("copy s3 object: duplicate etag")
				}
				etagSeen = true
				simpleCopyMetadata = token.Name.Local
			case "LastModified":
				if !s3XMLNamespaceAllowed(token.Name.Space) {
					return fmt.Errorf("copy s3 object: unexpected response namespace")
				}
				if lastModifiedSeen {
					return fmt.Errorf("copy s3 object: duplicate last-modified")
				}
				lastModifiedSeen = true
				simpleCopyMetadata = token.Name.Local
			case "Error":
				if !s3XMLNamespaceAllowed(token.Name.Space) {
					return fmt.Errorf("copy s3 object: unexpected response namespace")
				}
				simpleCopyMetadata = ""
				response, err := parseS3XMLErrorElement(decoder, token)
				if err != nil {
					if _, ok := err.(s3AmbiguousErrorFieldError); ok {
						return fmt.Errorf("copy s3 object: embedded error")
					}
					return fmt.Errorf("decode s3 copy response: %w", err)
				}
				preview := s3ErrorPreview(response.Code, response.Message, s3ErrorDetail("request-id", response.RequestID), s3ErrorDetail("host-id", response.HostID))
				if preview == "" {
					return fmt.Errorf("copy s3 object: embedded error")
				}
				return fmt.Errorf("copy s3 object: embedded error: %s", preview)
			default:
				if !s3XMLNamespaceAllowed(token.Name.Space) {
					return fmt.Errorf("copy s3 object: unexpected response namespace")
				}
				return fmt.Errorf("copy s3 object: unexpected response child %q", token.Name.Local)
			}
		case xml.EndElement:
			if rootDepth == 2 && simpleCopyMetadata == token.Name.Local {
				simpleCopyMetadata = ""
			}
			if rootDepth > 0 {
				rootDepth--
			}
		}
	}
}

func s3XMLNamespaceAllowed(value string) bool {
	return value == "" || value == s3XMLNamespace
}
