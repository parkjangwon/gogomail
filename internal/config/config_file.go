package config

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

func LoadFile(path string) (Config, error) {
	cfg := Load()
	if strings.TrimSpace(path) == "" {
		return cfg, nil
	}
	if err := applyYAMLFile(&cfg, path); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func applyYAMLFile(cfg *Config, path string) error {
	data, err := os.ReadFile(strings.TrimSpace(path))
	if err != nil {
		return fmt.Errorf("read config file: %w", err)
	}
	var values map[string]any
	if err := yaml.Unmarshal(data, &values); err != nil {
		return fmt.Errorf("parse config file: %w", err)
	}
	for key, value := range values {
		if err := applyYAMLConfigValue(cfg, key, value); err != nil {
			return err
		}
	}
	return nil
}

func applyYAMLConfigValue(cfg *Config, key string, value any) error {
	switch strings.TrimSpace(key) {
	case "environment":
		return setYAMLString(value, &cfg.Environment, key)
	case "http_addr":
		return setYAMLString(value, &cfg.HTTPAddr, key)
	case "smtp_addr":
		return setYAMLString(value, &cfg.SMTPAddr, key)
	case "inbound_smtp_addr":
		return setYAMLString(value, &cfg.InboundSMTPAddr, key)
	case "inbound_trusted_relays":
		return setYAMLStringSlice(value, &cfg.InboundTrustedRelays, key)
	case "imap_addr":
		return setYAMLString(value, &cfg.IMAPAddr, key)
	case "imap_tls_cert_file":
		return setYAMLString(value, &cfg.IMAPTLSCertFile, key)
	case "imap_tls_key_file":
		return setYAMLString(value, &cfg.IMAPTLSKeyFile, key)
	case "imap_allow_insecure_auth":
		return setYAMLBool(value, &cfg.IMAPAllowInsecureAuth, key)
	case "imap_max_connections":
		return setYAMLInt(value, &cfg.IMAPMaxConnections, key)
	case "imap_read_timeout":
		return setYAMLDuration(value, &cfg.IMAPReadTimeout, key)
	case "imap_write_timeout":
		return setYAMLDuration(value, &cfg.IMAPWriteTimeout, key)
	case "imap_idle_timeout":
		return setYAMLDuration(value, &cfg.IMAPIdleTimeout, key)
	case "pop3s_addr":
		return setYAMLString(value, &cfg.POP3SAddr, key)
	case "pop3_max_connections":
		return setYAMLInt(value, &cfg.POP3MaxConnections, key)
	case "caldav_addr":
		return setYAMLString(value, &cfg.CalDAVAddr, key)
	case "caldav_allow_insecure_auth":
		return setYAMLBool(value, &cfg.CalDAVAllowInsecureAuth, key)
	case "caldav_trust_forwarded_proto":
		return setYAMLBool(value, &cfg.CalDAVTrustForwardedProto, key)
	case "caldav_trusted_proxies":
		return setYAMLStringSlice(value, &cfg.CalDAVTrustedProxies, key)
	case "carddav_addr":
		return setYAMLString(value, &cfg.CardDAVAddr, key)
	case "carddav_allow_insecure_auth":
		return setYAMLBool(value, &cfg.CardDAVAllowInsecureAuth, key)
	case "carddav_trust_forwarded_proto":
		return setYAMLBool(value, &cfg.CardDAVTrustForwardedProto, key)
	case "carddav_trusted_proxies":
		return setYAMLStringSlice(value, &cfg.CardDAVTrustedProxies, key)
	case "webdav_addr":
		return setYAMLString(value, &cfg.WebDAVAddr, key)
	case "webdav_depth_infinity_enabled":
		return setYAMLBool(value, &cfg.WebDAVDepthInfinityEnabled, key)
	case "ldap_addr":
		return setYAMLString(value, &cfg.LDAPAddr, key)
	case "ldaps_addr":
		return setYAMLString(value, &cfg.LDAPSAddr, key)
	case "ldap_tls_cert_file":
		return setYAMLString(value, &cfg.LDAPTLSCertFile, key)
	case "ldap_tls_key_file":
		return setYAMLString(value, &cfg.LDAPTLSKeyFile, key)
	case "ldap_company_id":
		return setYAMLString(value, &cfg.LDAPCompanyID, key)
	case "ldap_base_domain":
		return setYAMLString(value, &cfg.LDAPBaseDomain, key)
	case "ldap_referral_urls":
		return setYAMLStringSlice(value, &cfg.LDAPReferralURLs, key)
	case "submission_addr":
		return setYAMLString(value, &cfg.SubmissionAddr, key)
	case "submission_smtps_addr":
		return setYAMLString(value, &cfg.SubmissionSMTPSAddr, key)
	case "submission_max_connections":
		return setYAMLInt(value, &cfg.SubmissionMaxConnections, key)
	case "submission_max_recipients":
		return setYAMLInt(value, &cfg.SubmissionMaxRecipients, key)
	case "submission_max_message_bytes":
		return setYAMLInt64(value, &cfg.SubmissionMaxMessageBytes, key)
	case "submission_add_received_header":
		return setYAMLBool(value, &cfg.SubmissionAddReceivedHeader, key)
	case "submission_support_smtputf8":
		return setYAMLBool(value, &cfg.SubmissionSupportSMTPUTF8, key)
	case "submission_support_requiretls":
		return setYAMLBool(value, &cfg.SubmissionSupportRequireTLS, key)
	case "submission_support_dsn":
		return setYAMLBool(value, &cfg.SubmissionSupportDSN, key)
	case "submission_support_binarymime":
		return setYAMLBool(value, &cfg.SubmissionSupportBinaryMIME, key)
	case "smtp_tls_cert_file":
		return setYAMLString(value, &cfg.SMTPTLSCertFile, key)
	case "smtp_tls_key_file":
		return setYAMLString(value, &cfg.SMTPTLSKeyFile, key)
	case "submission_allow_insecure_auth":
		return setYAMLBool(value, &cfg.SubmissionAllowInsecureAuth, key)
	case "database_url":
		return setYAMLString(value, &cfg.DatabaseURL, key)
	case "redis_addr":
		return setYAMLString(value, &cfg.RedisAddr, key)
	case "redis_password":
		return setYAMLString(value, &cfg.RedisPassword, key)
	case "auth_jwt_secret":
		return setYAMLString(value, &cfg.AuthJWTSecret, key)
	case "admin_token":
		return setYAMLString(value, &cfg.AdminToken, key)
	case "public_base_url":
		return setYAMLString(value, &cfg.PublicBaseURL, key)
	case "storage_backend":
		return setYAMLString(value, &cfg.StorageBackend, key)
	case "storage_backend_compat_labels":
		return setYAMLStringSlice(value, &cfg.StorageBackendCompatLabels, key)
	case "storage_s3_endpoint":
		return setYAMLString(value, &cfg.StorageS3Endpoint, key)
	case "storage_s3_region":
		return setYAMLString(value, &cfg.StorageS3Region, key)
	case "storage_s3_bucket":
		return setYAMLString(value, &cfg.StorageS3Bucket, key)
	case "storage_s3_prefix":
		return setYAMLString(value, &cfg.StorageS3Prefix, key)
	case "storage_s3_access_key_id":
		return setYAMLString(value, &cfg.StorageS3AccessKeyID, key)
	case "storage_s3_secret_access_key":
		return setYAMLString(value, &cfg.StorageS3SecretAccessKey, key)
	case "storage_s3_session_token":
		return setYAMLString(value, &cfg.StorageS3SessionToken, key)
	case "storage_s3_force_path_style":
		return setYAMLBool(value, &cfg.StorageS3ForcePathStyle, key)
	case "storage_s3_ca_cert_file":
		return setYAMLString(value, &cfg.StorageS3CACertFile, key)
	case "storage_s3_insecure_skip_verify":
		return setYAMLBool(value, &cfg.StorageS3InsecureSkipVerify, key)
	case "migration_dir":
		return setYAMLString(value, &cfg.MigrationDir, key)
	case "smtp_domain":
		return setYAMLString(value, &cfg.SMTPDomain, key)
	case "delivery_smtp_hello":
		return setYAMLString(value, &cfg.DeliverySMTPHello, key)
	case "smtp_max_connections":
		return setYAMLInt(value, &cfg.SMTPMaxConnections, key)
	case "smtp_auth_verification_enabled":
		return setYAMLBool(value, &cfg.SMTPAuthVerificationEnabled, key)
	case "smtp_authserv_id":
		return setYAMLString(value, &cfg.SMTPAuthservID, key)
	case "smtp_dmarc_enforcement":
		return setYAMLString(value, &cfg.SMTPDMARCEnforcement, key)
	case "smtp_max_dkim_verifications":
		return setYAMLInt(value, &cfg.SMTPMaxDKIMVerifications, key)
	case "mailstore_root", "storage_root":
		return setYAMLString(value, &cfg.MailstoreRoot, key)
	case "local_recipients":
		return setYAMLStringSlice(value, &cfg.LocalRecipients, key)
	case "metrics_backend":
		return setYAMLString(value, &cfg.MetricsBackend, key)
	case "attachment_scan_backend":
		return setYAMLString(value, &cfg.AttachmentScanBackend, key)
	case "attachment_scan_clamav_addr":
		return setYAMLString(value, &cfg.AttachmentScanClamAVAddr, key)
	case "attachment_scan_max_concurrency":
		return setYAMLInt(value, &cfg.AttachmentScanMaxConcurrency, key)
	case "attachment_scan_max_bytes":
		return setYAMLInt64(value, &cfg.AttachmentScanMaxBytes, key)
	case "attachment_scan_failure_threshold":
		return setYAMLInt(value, &cfg.AttachmentScanFailureThreshold, key)
	case "attachment_scan_circuit_open_duration":
		return setYAMLDuration(value, &cfg.AttachmentScanCircuitOpenDuration, key)
	case "attachment_scan_webhook_url":
		return setYAMLString(value, &cfg.AttachmentScanWebhookURL, key)
	case "attachment_scan_webhook_token":
		return setYAMLString(value, &cfg.AttachmentScanWebhookToken, key)
	case "attachment_scan_timeout":
		return setYAMLDuration(value, &cfg.AttachmentScanTimeout, key)
	case "push_notification_backend":
		return setYAMLString(value, &cfg.PushNotifyBackend, key)
	case "push_notification_webhook_url":
		return setYAMLString(value, &cfg.PushNotifyWebhookURL, key)
	case "push_notification_webhook_token":
		return setYAMLString(value, &cfg.PushNotifyWebhookToken, key)
	case "push_notification_webhook_timeout":
		return setYAMLDuration(value, &cfg.PushNotifyWebhookTimeout, key)
	case "push_notification_consumer_group":
		return setYAMLString(value, &cfg.PushNotifyConsumerGroup, key)
	case "push_notification_consumer_name":
		return setYAMLString(value, &cfg.PushNotifyConsumerName, key)
	case "push_notification_consumer_count":
		return setYAMLInt(value, &cfg.PushNotifyConsumerCount, key)
	case "push_notification_consumer_block":
		return setYAMLDuration(value, &cfg.PushNotifyConsumerBlock, key)
	case "apns_key_id":
		return setYAMLString(value, &cfg.APNsKeyID, key)
	case "apns_team_id":
		return setYAMLString(value, &cfg.APNsTeamID, key)
	case "apns_private_key":
		return setYAMLString(value, &cfg.APNsPrivateKey, key)
	case "apns_bundle_id":
		return setYAMLString(value, &cfg.APNsBundleID, key)
	case "webpush_vapid_public_key":
		return setYAMLString(value, &cfg.WebPushVAPIDPublicKey, key)
	case "webpush_vapid_private_key":
		return setYAMLString(value, &cfg.WebPushVAPIDPrivateKey, key)
	case "webpush_contact_email":
		return setYAMLString(value, &cfg.WebPushContactEmail, key)
	case "webhook_dispatch_enabled":
		return setYAMLBool(value, &cfg.WebhookDispatchEnabled, key)
	case "api_metering_backend":
		return setYAMLString(value, &cfg.APIMeteringBackend, key)
	case "api_metering_timeout":
		return setYAMLDuration(value, &cfg.APIMeteringTimeout, key)
	case "api_metering_aggregate_backend":
		return setYAMLString(value, &cfg.APIMeteringAggregateBackend, key)
	case "api_metering_stream":
		return setYAMLString(value, &cfg.APIMeteringStream, key)
	case "api_metering_consumer_group":
		return setYAMLString(value, &cfg.APIMeteringConsumerGroup, key)
	case "api_metering_consumer_name":
		return setYAMLString(value, &cfg.APIMeteringConsumerName, key)
	case "api_metering_consumer_count":
		return setYAMLInt(value, &cfg.APIMeteringConsumerCount, key)
	case "api_metering_consumer_block":
		return setYAMLDuration(value, &cfg.APIMeteringConsumerBlock, key)
	case "search_index_backend":
		return setYAMLString(value, &cfg.SearchIndexBackend, key)
	case "search_index_max_body_bytes":
		return setYAMLInt64(value, &cfg.SearchIndexMaxBodyBytes, key)
	case "delivery_throttle_enabled":
		return setYAMLBool(value, &cfg.DeliveryThrottleEnabled, key)
	case "delivery_throttle_backend":
		return setYAMLString(value, &cfg.DeliveryThrottleBackend, key)
	case "delivery_default_concurrency":
		return setYAMLInt(value, &cfg.DeliveryDefaultConcurrency, key)
	case "delivery_farm_concurrency":
		return setYAMLIntMap(value, &cfg.DeliveryFarmConcurrency, key)
	case "delivery_domain_concurrency":
		return setYAMLIntMap(value, &cfg.DeliveryDomainConcurrency, key)
	case "delivery_domain_backoff_enabled":
		return setYAMLBool(value, &cfg.DeliveryDomainBackoffEnabled, key)
	case "delivery_domain_backoff_backend":
		return setYAMLString(value, &cfg.DeliveryDomainBackoffBackend, key)
	case "delivery_domain_backoff_scope":
		return setYAMLString(value, &cfg.DeliveryDomainBackoffScope, key)
	case "delivery_domain_backoff_base_delay":
		return setYAMLDuration(value, &cfg.DeliveryDomainBackoffBaseDelay, key)
	case "delivery_domain_backoff_max_delay":
		return setYAMLDuration(value, &cfg.DeliveryDomainBackoffMaxDelay, key)
	case "farm_coordinator_backend":
		return setYAMLString(value, &cfg.FarmCoordinatorBackend, key)
	case "farm_coordinator_node_id":
		return setYAMLString(value, &cfg.FarmCoordinatorNodeID, key)
	case "farm_coordinator_heartbeat_ttl":
		return setYAMLDuration(value, &cfg.FarmCoordinatorHeartbeatTTL, key)
	case "farm_coordinator_job_visibility_timeout":
		return setYAMLDuration(value, &cfg.FarmCoordinatorJobVisibilityTimeout, key)
	case "delivery_smarthost":
		return setYAMLString(value, &cfg.DeliverySmartHost, key)
	case "delivery_smarthost_port":
		return setYAMLInt(value, &cfg.DeliverySmartHostPort, key)
	case "delivery_smarthost_tls_mode":
		return setYAMLString(value, &cfg.DeliverySmartHostTLSMode, key)
	case "delivery_smarthost_implicit_tls":
		return setYAMLBool(value, &cfg.DeliverySmartHostImplicitTLS, key)
	case "delivery_smarthost_username":
		return setYAMLString(value, &cfg.DeliverySmartHostUsername, key)
	case "cors_allowed_origins":
		return setYAMLString(value, &cfg.CORSAllowedOrigins, key)
	case "metrics_addr":
		return setYAMLString(value, &cfg.MetricsAddr, key)
	default:
		return fmt.Errorf("config file has unsupported key %q", key)
	}
}

func setYAMLString(value any, dest *string, key string) error {
	text, ok := value.(string)
	if !ok {
		return fmt.Errorf("config key %q must be a string", key)
	}
	*dest = text
	return nil
}

func setYAMLBool(value any, dest *bool, key string) error {
	parsed, ok := value.(bool)
	if !ok {
		return fmt.Errorf("config key %q must be a boolean", key)
	}
	*dest = parsed
	return nil
}

func setYAMLInt(value any, dest *int, key string) error {
	parsed, err := yamlInt64(value, key)
	if err != nil {
		return err
	}
	if parsed < math.MinInt || parsed > math.MaxInt {
		return fmt.Errorf("config key %q is outside int range", key)
	}
	*dest = int(parsed)
	return nil
}

func setYAMLInt64(value any, dest *int64, key string) error {
	parsed, err := yamlInt64(value, key)
	if err != nil {
		return err
	}
	*dest = parsed
	return nil
}

func yamlInt64(value any, key string) (int64, error) {
	switch typed := value.(type) {
	case int:
		return int64(typed), nil
	case int64:
		return typed, nil
	case uint64:
		if typed > uint64(^uint64(0)>>1) {
			return 0, fmt.Errorf("config key %q is outside int64 range", key)
		}
		return int64(typed), nil
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("config key %q must be an integer", key)
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("config key %q must be an integer", key)
	}
}

func setYAMLDuration(value any, dest *time.Duration, key string) error {
	text, ok := value.(string)
	if !ok {
		return fmt.Errorf("config key %q must be a duration string", key)
	}
	parsed, err := time.ParseDuration(strings.TrimSpace(text))
	if err != nil {
		return fmt.Errorf("config key %q must be a valid duration: %w", key, err)
	}
	*dest = parsed
	return nil
}

func setYAMLStringSlice(value any, dest *[]string, key string) error {
	switch typed := value.(type) {
	case string:
		*dest = splitCSV(typed)
		return nil
	case []any:
		values := make([]string, 0, len(typed))
		for _, item := range typed {
			text, ok := item.(string)
			if !ok {
				return fmt.Errorf("config key %q must contain only strings", key)
			}
			if trimmed := strings.TrimSpace(text); trimmed != "" {
				values = append(values, trimmed)
			}
		}
		*dest = values
		return nil
	default:
		return fmt.Errorf("config key %q must be a string list", key)
	}
}

func setYAMLIntMap(value any, dest *map[string]int, key string) error {
	switch typed := value.(type) {
	case string:
		parsed, err := parseYAMLStringIntMap(typed, key)
		if err != nil {
			return err
		}
		*dest = parsed
		return nil
	case map[string]any:
		result := make(map[string]int, len(typed))
		for mapKey, mapValue := range typed {
			parsed, err := yamlInt64(mapValue, key+"."+mapKey)
			if err != nil {
				return err
			}
			if parsed < math.MinInt || parsed > math.MaxInt {
				return fmt.Errorf("config key %q is outside int range", key+"."+mapKey)
			}
			result[strings.TrimSpace(mapKey)] = int(parsed)
		}
		*dest = result
		return nil
	case map[any]any:
		result := make(map[string]int, len(typed))
		for mapKey, mapValue := range typed {
			textKey, ok := mapKey.(string)
			if !ok {
				return fmt.Errorf("config key %q map keys must be strings", key)
			}
			parsed, err := yamlInt64(mapValue, key+"."+textKey)
			if err != nil {
				return err
			}
			if parsed < math.MinInt || parsed > math.MaxInt {
				return fmt.Errorf("config key %q is outside int range", key+"."+textKey)
			}
			result[strings.TrimSpace(textKey)] = int(parsed)
		}
		*dest = result
		return nil
	default:
		return fmt.Errorf("config key %q must be an integer map", key)
	}
}

func parseYAMLStringIntMap(raw string, key string) (map[string]int, error) {
	result := make(map[string]int)
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		name, value, ok := strings.Cut(part, "=")
		if !ok {
			return nil, fmt.Errorf("config key %q must use name=value entries", key)
		}
		parsed, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			return nil, fmt.Errorf("config key %q has invalid integer for %q", key, strings.TrimSpace(name))
		}
		result[strings.TrimSpace(name)] = parsed
	}
	return result, nil
}
