package storage

import "strings"

type BackendCapabilities struct {
	ContractVersion       string   `json:"contract_version"`
	ConfiguredBackend     string   `json:"configured_backend"`
	BackendClass          string   `json:"backend_class"`
	ActiveLabels          []string `json:"active_labels"`
	Operations            []string `json:"operations"`
	LocalFilesystem       bool     `json:"local_filesystem"`
	S3Compatible          bool     `json:"s3_compatible"`
	PathStyleAddressing   bool     `json:"path_style_addressing"`
	CompatLabelsEnabled   bool     `json:"compat_labels_enabled"`
	ReadinessProbe        bool     `json:"readiness_probe"`
	EndpointOrigin        string   `json:"endpoint_origin,omitempty"`
	Bucket                string   `json:"bucket,omitempty"`
	Prefix                string   `json:"prefix,omitempty"`
	Region                string   `json:"region,omitempty"`
	SecretsRedacted       bool     `json:"secrets_redacted"`
	SupportsBackendSwitch bool     `json:"supports_backend_switch"`
	SupportsLocalNFS      bool     `json:"supports_local_nfs"`
	SupportsMinIO         bool     `json:"supports_minio"`
	SupportsAWSCompatible bool     `json:"supports_aws_compatible"`
	RequiresByteMigration bool     `json:"requires_byte_migration"`
}

func SupportMatrixForLabels(labels []string) (supportsLocalNFS bool, supportsMinIO bool, supportsAWSCompatible bool) {
	for _, label := range labels {
		label = strings.ToLower(strings.TrimSpace(label))
		switch label {
		case "local", "nfs":
			supportsLocalNFS = true
		case "minio":
			supportsMinIO = true
			supportsAWSCompatible = true
		case "s3":
			supportsAWSCompatible = true
		}
	}
	return supportsLocalNFS, supportsMinIO, supportsAWSCompatible
}
