package app

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"encoding/base64"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	gosmtp "github.com/emersion/go-smtp"
	"github.com/gogomail/gogomail/internal/apikeys"
	"github.com/gogomail/gogomail/internal/apimeter"
	"github.com/gogomail/gogomail/internal/auth"
	"github.com/gogomail/gogomail/internal/config"
	"github.com/gogomail/gogomail/internal/delivery"
	"github.com/gogomail/gogomail/internal/directory"
	"github.com/gogomail/gogomail/internal/drive"
	"github.com/gogomail/gogomail/internal/eventstream"
	"github.com/gogomail/gogomail/internal/httpapi"
	"github.com/gogomail/gogomail/internal/imapgw"
	"github.com/gogomail/gogomail/internal/maildb"
	"github.com/gogomail/gogomail/internal/storage"
)

func TestOrgChartServiceForDBSatisfiesHTTPRouteInterface(t *testing.T) {
	t.Parallel()

	var _ httpapi.OrgChartService = orgChartServiceForDB(nil)
	if orgChartServiceForDB(nil) == nil {
		t.Fatal("orgChartServiceForDB returned nil")
	}
}

func TestSMTPTLSConfigRequiresCertAndKeyTogether(t *testing.T) {
	t.Parallel()

	if _, err := smtpTLSConfig(config.Config{SMTPTLSCertFile: "cert.pem"}); err == nil {
		t.Fatal("smtpTLSConfig accepted certificate without key")
	}
	if _, err := smtpTLSConfig(config.Config{SMTPTLSKeyFile: "key.pem"}); err == nil {
		t.Fatal("smtpTLSConfig accepted key without certificate")
	}
}

func TestDeliveryDomainBackoffFromConfig(t *testing.T) {
	t.Parallel()

	if got := deliveryDomainBackoffFromConfig(config.Config{}, nil); got != nil {
		t.Fatalf("deliveryDomainBackoffFromConfig disabled = %T, want nil", got)
	}
	got := deliveryDomainBackoffFromConfig(config.Config{
		DeliveryDomainBackoffEnabled:   true,
		DeliveryDomainBackoffBackend:   "local",
		DeliveryDomainBackoffScope:     "farm_domain",
		DeliveryDomainBackoffBaseDelay: time.Minute,
		DeliveryDomainBackoffMaxDelay:  time.Hour,
	}, nil)
	if got == nil {
		t.Fatal("deliveryDomainBackoffFromConfig enabled = nil")
	}
	redisBackoff := deliveryDomainBackoffFromConfig(config.Config{
		DeliveryDomainBackoffEnabled:   true,
		DeliveryDomainBackoffBackend:   "redis",
		DeliveryDomainBackoffScope:     "farm_domain",
		DeliveryDomainBackoffBaseDelay: time.Minute,
		DeliveryDomainBackoffMaxDelay:  time.Hour,
	}, nil)
	if redisBackoff == nil {
		t.Fatal("deliveryDomainBackoffFromConfig redis = nil")
	}
}

func TestSMTPTLSConfigAllowsNoTLSFiles(t *testing.T) {
	t.Parallel()

	tlsConfig, err := smtpTLSConfig(config.Config{})
	if err != nil {
		t.Fatalf("smtpTLSConfig returned error: %v", err)
	}
	if tlsConfig != nil {
		t.Fatal("smtpTLSConfig returned config without TLS files")
	}
}

func TestIMAPTLSConfigRequiresCertAndKeyTogether(t *testing.T) {
	t.Parallel()

	if _, err := imapTLSConfig(config.Config{IMAPTLSCertFile: "cert.pem"}); err == nil {
		t.Fatal("imapTLSConfig accepted certificate without key")
	}
	if _, err := imapTLSConfig(config.Config{IMAPTLSKeyFile: "key.pem"}); err == nil {
		t.Fatal("imapTLSConfig accepted key without certificate")
	}
}

func TestIMAPTLSServerNameUsesListenerHost(t *testing.T) {
	t.Parallel()

	if got := imapTLSServerName(config.Config{IMAPAddr: "imap.example.com:993", SMTPDomain: "smtp.example.com"}); got != "imap.example.com" {
		t.Fatalf("server name = %q, want imap listener host", got)
	}
	if got := imapTLSServerName(config.Config{IMAPAddr: ":1143", SMTPDomain: "smtp.example.com"}); got != "smtp.example.com" {
		t.Fatalf("server name = %q, want SMTP domain fallback", got)
	}
}

func TestLDAPTLSConfigRequiresCertAndKeyTogether(t *testing.T) {
	t.Parallel()

	if _, err := ldapTLSConfig(config.Config{LDAPTLSCertFile: "cert.pem"}); err == nil {
		t.Fatal("ldapTLSConfig accepted certificate without key")
	}
	if _, err := ldapTLSConfig(config.Config{LDAPTLSKeyFile: "key.pem"}); err == nil {
		t.Fatal("ldapTLSConfig accepted key without certificate")
	}
}

func TestLDAPTLSConfigAllowsNoTLSFiles(t *testing.T) {
	t.Parallel()

	tlsConfig, err := ldapTLSConfig(config.Config{})
	if err != nil {
		t.Fatalf("ldapTLSConfig returned error: %v", err)
	}
	if tlsConfig != nil {
		t.Fatal("ldapTLSConfig returned config without TLS files")
	}
}

func TestLDAPFilterToQueryExtractsDirectorySearchAttributes(t *testing.T) {
	t.Parallel()

	if got := ldapFilterToQuery("(cn=*Alice*)"); got != "Alice" {
		t.Fatalf("ldapFilterToQuery cn = %q, want Alice", got)
	}
	if got := ldapFilterToQuery("(ou=Research)"); got != "Research" {
		t.Fatalf("ldapFilterToQuery ou = %q, want Research", got)
	}
	if got := ldapFilterToQuery("(description=room)"); got != "room" {
		t.Fatalf("ldapFilterToQuery description = %q, want room", got)
	}
	if got := ldapFilterToQuery("(sAMAccountName=alice)"); got != "alice" {
		t.Fatalf("ldapFilterToQuery sAMAccountName = %q, want alice", got)
	}
	if got := ldapFilterToQuery("(userPrincipalName=alice@example.com)"); got != "alice@example.com" {
		t.Fatalf("ldapFilterToQuery userPrincipalName = %q, want alice@example.com", got)
	}
	if got := ldapFilterToQuery("(name=*Alice*)"); got != "Alice" {
		t.Fatalf("ldapFilterToQuery name = %q, want Alice", got)
	}
	if got := ldapFilterToQuery("(canonicalName=example.com/users/alice)"); got != "alice" {
		t.Fatalf("ldapFilterToQuery canonicalName = %q, want alice", got)
	}
	if got := ldapFilterToQuery("(mailNickname=alice)"); got != "alice" {
		t.Fatalf("ldapFilterToQuery mailNickname = %q, want alice", got)
	}
	if got := ldapFilterToQuery("(proxyAddresses=SMTP:alice@example.com)"); got != "SMTP:alice@example.com" {
		t.Fatalf("ldapFilterToQuery proxyAddresses = %q, want SMTP:alice@example.com", got)
	}
	if got := ldapFilterToQuery("(objectClass=person)"); got != "" {
		t.Fatalf("ldapFilterToQuery objectClass = %q, want empty", got)
	}
}

func TestLDAPPrincipalDNUsesKindSpecificSubtrees(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		p    directory.Principal
		want string
	}{
		{
			name: "user",
			p:    directory.Principal{ID: "user-1", Kind: directory.PrincipalKindUser},
			want: "uid=user-1,ou=users,dc=example,dc=com",
		},
		{
			name: "organization",
			p:    directory.Principal{ID: "org-1", Kind: directory.PrincipalKindOrganization},
			want: "ou=org-1,ou=organizations,dc=example,dc=com",
		},
		{
			name: "group",
			p:    directory.Principal{ID: "group-1", Kind: directory.PrincipalKindGroup},
			want: "cn=group-1,ou=groups,dc=example,dc=com",
		},
		{
			name: "resource",
			p:    directory.Principal{ID: "room-1", Kind: directory.PrincipalKindResource},
			want: "cn=room-1,ou=resources,dc=example,dc=com",
		},
		{
			name: "escaped user",
			p:    directory.Principal{ID: " user,1+ops ", Kind: directory.PrincipalKindUser},
			want: `uid=\20user\2c1\2bops\20,ou=users,dc=example,dc=com`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ldapPrincipalDN(tt.p, "dc=example,dc=com"); got != tt.want {
				t.Fatalf("ldapPrincipalDN = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLDAPPrincipalKindIDDNUsesKindSpecificSubtrees(t *testing.T) {
	t.Parallel()

	if got := ldapPrincipalKindIDDN(directory.PrincipalKindUser, "user-1", "dc=example,dc=com"); got != "uid=user-1,ou=users,dc=example,dc=com" {
		t.Fatalf("user member DN = %q", got)
	}
	if got := ldapPrincipalKindIDDN(directory.PrincipalKindOrganization, "org-1", "dc=example,dc=com"); got != "ou=org-1,ou=organizations,dc=example,dc=com" {
		t.Fatalf("organization member DN = %q", got)
	}
	if got := ldapPrincipalKindIDDN(directory.PrincipalKindGroup, "group-1", "dc=example,dc=com"); got != "cn=group-1,ou=groups,dc=example,dc=com" {
		t.Fatalf("group member DN = %q", got)
	}
	if got := ldapPrincipalKindIDDN(directory.PrincipalKindResource, "room-1", "dc=example,dc=com"); got != "cn=room-1,ou=resources,dc=example,dc=com" {
		t.Fatalf("resource member DN = %q", got)
	}
}

func TestLDAPShouldExpandGroupMembersOnlyWhenRequested(t *testing.T) {
	t.Parallel()

	if !ldapShouldExpandGroupMembers(nil) {
		t.Fatal("empty attrs should expand default user attributes")
	}
	if !ldapShouldExpandGroupMembers([]string{"cn", "member"}) {
		t.Fatal("explicit member attr should expand group members")
	}
	if !ldapShouldExpandGroupMembers([]string{"*"}) {
		t.Fatal("* selector should expand group members")
	}
	if ldapShouldExpandGroupMembers([]string{"cn", "displayName"}) {
		t.Fatal("narrow non-member attrs should not expand group members")
	}
	if ldapShouldExpandGroupMembers([]string{"+"}) {
		t.Fatal("+ operational selector should not expand group members")
	}
	if ldapShouldExpandGroupMembers([]string{"1.1"}) {
		t.Fatal("1.1 no-attrs selector should not expand group members")
	}
}

func TestLDAPShouldExpandMemberOfOnlyWhenRequested(t *testing.T) {
	t.Parallel()

	if !ldapShouldExpandMemberOf(nil) {
		t.Fatal("empty attrs should expand default memberOf attributes")
	}
	if !ldapShouldExpandMemberOf([]string{"cn", "memberOf"}) {
		t.Fatal("explicit memberOf attr should expand reverse group memberships")
	}
	if !ldapShouldExpandMemberOf([]string{"*"}) {
		t.Fatal("* selector should expand reverse group memberships")
	}
	if ldapShouldExpandMemberOf([]string{"cn", "displayName"}) {
		t.Fatal("narrow non-memberOf attrs should not expand reverse group memberships")
	}
	if ldapShouldExpandMemberOf([]string{"+"}) {
		t.Fatal("+ operational selector should not expand reverse group memberships")
	}
	if ldapShouldExpandMemberOf([]string{"1.1"}) {
		t.Fatal("1.1 no-attrs selector should not expand reverse group memberships")
	}
}

func TestLDAPPrincipalFromDNUsesKindSpecificSubtrees(t *testing.T) {
	t.Parallel()

	tests := []struct {
		dn       string
		wantKind string
		wantID   string
		wantOK   bool
	}{
		{dn: "uid=user-1,ou=users,dc=example,dc=com", wantKind: directory.PrincipalKindUser, wantID: "user-1", wantOK: true},
		{dn: "ou=org-1,ou=organizations,dc=example,dc=com", wantKind: directory.PrincipalKindOrganization, wantID: "org-1", wantOK: true},
		{dn: "cn=group-1,ou=groups,dc=example,dc=com", wantKind: directory.PrincipalKindGroup, wantID: "group-1", wantOK: true},
		{dn: "cn=room-1,ou=resources,dc=example,dc=com", wantKind: directory.PrincipalKindResource, wantID: "room-1", wantOK: true},
		{dn: `uid=\20user\2c1\2bops\20,ou=users,dc=example,dc=com`, wantKind: directory.PrincipalKindUser, wantID: " user,1+ops ", wantOK: true},
		{dn: "cn=room-1,ou=users,dc=example,dc=com", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.dn, func(t *testing.T) {
			gotKind, gotID, gotOK := ldapPrincipalFromDN(tt.dn)
			if gotOK != tt.wantOK || gotKind != tt.wantKind || gotID != tt.wantID {
				t.Fatalf("ldapPrincipalFromDN = %q/%q/%v, want %q/%q/%v", gotKind, gotID, gotOK, tt.wantKind, tt.wantID, tt.wantOK)
			}
		})
	}
}

type fakeStorageChecker struct {
	err error
}

func (f fakeStorageChecker) Check(context.Context) error {
	return f.err
}

func TestStorageReadinessCheckReportsProbeStatus(t *testing.T) {
	t.Parallel()

	ok := storageReadinessCheck("mail_storage", fakeStorageChecker{})(context.Background())
	if ok.Name != "mail_storage" || ok.Status != "ok" || ok.Detail == "" {
		t.Fatalf("ok check = %+v", ok)
	}
	failed := storageReadinessCheck("mail_storage", fakeStorageChecker{err: context.Canceled})(context.Background())
	if failed.Name != "mail_storage" || failed.Status != "error" || !strings.Contains(failed.Detail, "context canceled") {
		t.Fatalf("failed check = %+v", failed)
	}
}

func TestNewHTTPServerUsesConfiguredOperationalGuardrails(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		HTTPAddr:              ":18080",
		HTTPReadTimeout:       45 * time.Second,
		HTTPWriteTimeout:      90 * time.Second,
		HTTPIdleTimeout:       75 * time.Second,
		HTTPReadHeaderTimeout: 3 * time.Second,
		HTTPMaxHeaderBytes:    32 << 10,
	}
	handler := http.NewServeMux()
	server := newHTTPServer(cfg, handler)
	if server.Addr != ":18080" || server.Handler != handler {
		t.Fatalf("server identity = addr:%q handler:%T", server.Addr, server.Handler)
	}
	if server.ReadTimeout != 45*time.Second ||
		server.WriteTimeout != 90*time.Second ||
		server.IdleTimeout != 75*time.Second ||
		server.ReadHeaderTimeout != 3*time.Second ||
		server.MaxHeaderBytes != 32<<10 {
		t.Fatalf("server guardrails = read:%s write:%s idle:%s readHeader:%s maxHeader:%d",
			server.ReadTimeout,
			server.WriteTimeout,
			server.IdleTimeout,
			server.ReadHeaderTimeout,
			server.MaxHeaderBytes,
		)
	}
}

func TestNewCalDAVHTTPServerUsesDedicatedAddressAndHTTPGuardrails(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		CalDAVAddr:            " :18081 ",
		HTTPReadTimeout:       11 * time.Second,
		HTTPWriteTimeout:      12 * time.Second,
		HTTPIdleTimeout:       13 * time.Second,
		HTTPReadHeaderTimeout: 2 * time.Second,
		HTTPMaxHeaderBytes:    16 << 10,
	}
	handler := http.NewServeMux()
	server := newCalDAVHTTPServer(cfg, handler)
	if server.Addr != ":18081" || server.Handler != handler {
		t.Fatalf("server identity = addr:%q handler:%T", server.Addr, server.Handler)
	}
	if server.ReadTimeout != 11*time.Second ||
		server.WriteTimeout != 12*time.Second ||
		server.IdleTimeout != 13*time.Second ||
		server.ReadHeaderTimeout != 2*time.Second ||
		server.MaxHeaderBytes != 16<<10 {
		t.Fatalf("server guardrails = read:%s write:%s idle:%s readHeader:%s maxHeader:%d",
			server.ReadTimeout,
			server.WriteTimeout,
			server.IdleTimeout,
			server.ReadHeaderTimeout,
			server.MaxHeaderBytes,
		)
	}
}

func TestNewCardDAVHTTPServerUsesDedicatedAddressAndHTTPGuardrails(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		CardDAVAddr:           " :18082 ",
		HTTPReadTimeout:       11 * time.Second,
		HTTPWriteTimeout:      12 * time.Second,
		HTTPIdleTimeout:       13 * time.Second,
		HTTPReadHeaderTimeout: 2 * time.Second,
		HTTPMaxHeaderBytes:    16 << 10,
	}
	handler := http.NewServeMux()
	server := newCardDAVHTTPServer(cfg, handler)
	if server.Addr != ":18082" || server.Handler != handler {
		t.Fatalf("server identity = addr:%q handler:%T", server.Addr, server.Handler)
	}
	if server.ReadTimeout != 11*time.Second ||
		server.WriteTimeout != 12*time.Second ||
		server.IdleTimeout != 13*time.Second ||
		server.ReadHeaderTimeout != 2*time.Second ||
		server.MaxHeaderBytes != 16<<10 {
		t.Fatalf("server guardrails = read:%s write:%s idle:%s readHeader:%s maxHeader:%d",
			server.ReadTimeout,
			server.WriteTimeout,
			server.IdleTimeout,
			server.ReadHeaderTimeout,
			server.MaxHeaderBytes,
		)
	}
}

func TestObjectStoreForConfigRejectsUnsupportedBackend(t *testing.T) {
	t.Parallel()

	store, err := objectStoreForConfig(config.Config{StorageBackend: "swift", MailstoreRoot: t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), "unsupported storage backend") {
		t.Fatalf("store = %+v err = %v", store, err)
	}
}

func TestObjectStoreForConfigBuildsNFSAliasBackend(t *testing.T) {
	t.Parallel()

	store, err := objectStoreForConfig(config.Config{StorageBackend: " nfs ", MailstoreRoot: t.TempDir()})
	if err != nil {
		t.Fatalf("objectStoreForConfig returned error: %v", err)
	}
	if _, ok := store.(*storage.LocalStore); !ok {
		t.Fatalf("store = %T, want *storage.LocalStore", store)
	}
}

func TestStorageCapabilitiesForConfigDescribesLocalBackend(t *testing.T) {
	t.Parallel()

	capabilities := storageCapabilitiesForConfig(config.Config{
		StorageBackend:             " LOCAL ",
		StorageBackendCompatLabels: []string{"s3", " local "},
	})
	if capabilities.ConfiguredBackend != "local" || capabilities.BackendClass != "local" || !capabilities.LocalFilesystem || capabilities.S3Compatible {
		t.Fatalf("capabilities = %+v", capabilities)
	}
	if len(capabilities.ActiveLabels) != 3 || capabilities.ActiveLabels[0] != "local" || capabilities.ActiveLabels[1] != "nfs" || capabilities.ActiveLabels[2] != "s3" {
		t.Fatalf("active labels = %#v", capabilities.ActiveLabels)
	}
	if !capabilities.CompatLabelsEnabled || !capabilities.ReadinessProbe || !capabilities.SecretsRedacted || !capabilities.SupportsBackendSwitch {
		t.Fatalf("capabilities = %+v", capabilities)
	}
	if !capabilities.SupportsLocalNFS || capabilities.SupportsMinIO || !capabilities.SupportsAWSCompatible {
		t.Fatalf("support flags = local_nfs:%v minio:%v aws:%v", capabilities.SupportsLocalNFS, capabilities.SupportsMinIO, capabilities.SupportsAWSCompatible)
	}
}

func TestStorageCapabilitiesForConfigPreservesExtensibleCompatLabels(t *testing.T) {
	t.Parallel()

	capabilities := storageCapabilitiesForConfig(config.Config{
		StorageBackend:             "local",
		StorageBackendCompatLabels: []string{" azure_edge-1.compat ", "NFS"},
	})
	if len(capabilities.ActiveLabels) != 3 || capabilities.ActiveLabels[0] != "azure_edge-1.compat" || capabilities.ActiveLabels[1] != "local" || capabilities.ActiveLabels[2] != "nfs" {
		t.Fatalf("active labels = %#v", capabilities.ActiveLabels)
	}
	if !capabilities.SupportsLocalNFS || capabilities.SupportsMinIO || capabilities.SupportsAWSCompatible {
		t.Fatalf("support flags = local_nfs:%v minio:%v aws:%v", capabilities.SupportsLocalNFS, capabilities.SupportsMinIO, capabilities.SupportsAWSCompatible)
	}
}

func TestStorageCapabilitiesForConfigDescribesNFSAliasBackend(t *testing.T) {
	t.Parallel()

	capabilities := storageCapabilitiesForConfig(config.Config{StorageBackend: " nfs "})
	if capabilities.ConfiguredBackend != "nfs" || capabilities.BackendClass != "local" || !capabilities.LocalFilesystem || capabilities.S3Compatible {
		t.Fatalf("capabilities = %+v", capabilities)
	}
	if len(capabilities.ActiveLabels) != 2 || capabilities.ActiveLabels[0] != "local" || capabilities.ActiveLabels[1] != "nfs" {
		t.Fatalf("active labels = %#v", capabilities.ActiveLabels)
	}
	if !capabilities.SupportsLocalNFS || capabilities.SupportsMinIO || capabilities.SupportsAWSCompatible {
		t.Fatalf("support flags = local_nfs:%v minio:%v aws:%v", capabilities.SupportsLocalNFS, capabilities.SupportsMinIO, capabilities.SupportsAWSCompatible)
	}
}

func TestStorageCapabilitiesForConfigDescribesS3CompatibleBackend(t *testing.T) {
	t.Parallel()

	capabilities := storageCapabilitiesForConfig(config.Config{
		StorageBackend:           "s3",
		StorageS3Endpoint:        "https://s3.us-east-1.amazonaws.com/base+proxy",
		StorageS3Region:          "us-east-1",
		StorageS3Bucket:          "gogomail.prod",
		StorageS3Prefix:          "/mail/",
		StorageS3AccessKeyID:     "AKIAEXAMPLE",
		StorageS3SecretAccessKey: "secret",
	})
	if capabilities.ConfiguredBackend != "s3" || capabilities.BackendClass != "s3_compatible" || !capabilities.S3Compatible || !capabilities.PathStyleAddressing {
		t.Fatalf("capabilities = %+v", capabilities)
	}
	if capabilities.EndpointOrigin != "https://s3.us-east-1.amazonaws.com/base+proxy" || capabilities.Bucket != "gogomail.prod" || capabilities.Prefix != "mail" || capabilities.Region != "us-east-1" {
		t.Fatalf("sanitized S3 profile = %+v", capabilities)
	}
	if capabilities.SupportsLocalNFS || !capabilities.SupportsMinIO || !capabilities.SupportsAWSCompatible {
		t.Fatalf("support flags = local_nfs:%v minio:%v aws:%v", capabilities.SupportsLocalNFS, capabilities.SupportsMinIO, capabilities.SupportsAWSCompatible)
	}
	for _, leaked := range []string{"AKIAEXAMPLE", "secret"} {
		if strings.Contains(capabilities.EndpointOrigin+capabilities.Bucket+capabilities.Prefix+capabilities.Region, leaked) {
			t.Fatalf("storage capabilities leaked secret-like value %q: %+v", leaked, capabilities)
		}
	}
}

func TestStorageCapabilitiesForConfigUsesPathStyleForLocalS3Endpoints(t *testing.T) {
	t.Parallel()

	for _, endpoint := range []string{"http://localhost:19000", "http://127.0.0.1:19000", "http://[::1]:19000"} {
		t.Run(endpoint, func(t *testing.T) {
			t.Parallel()

			capabilities := storageCapabilitiesForConfig(config.Config{
				StorageBackend:    "s3",
				StorageS3Endpoint: endpoint,
				StorageS3Region:   "us-east-1",
				StorageS3Bucket:   "gogomail",
			})
			if !capabilities.S3Compatible || !capabilities.PathStyleAddressing {
				t.Fatalf("capabilities = %+v", capabilities)
			}
			if capabilities.EndpointOrigin != endpoint {
				t.Fatalf("endpoint origin = %q, want %q", capabilities.EndpointOrigin, endpoint)
			}
		})
	}
}

func TestObjectStoreForConfigBuildsS3Backend(t *testing.T) {
	t.Parallel()

	store, err := objectStoreForConfig(config.Config{
		StorageBackend:           "s3",
		StorageS3Endpoint:        "http://localhost:9000",
		StorageS3Region:          "us-east-1",
		StorageS3Bucket:          "gogomail",
		StorageS3AccessKeyID:     "access",
		StorageS3SecretAccessKey: "secret",
		StorageS3ForcePathStyle:  true,
	})
	if err != nil {
		t.Fatalf("objectStoreForConfig returned error: %v", err)
	}
	if _, ok := store.(*storage.S3Store); !ok {
		t.Fatalf("store = %T, want *storage.S3Store", store)
	}
}

func TestObjectStoreForConfigBuildsMinIOBackend(t *testing.T) {
	t.Parallel()

	store, err := objectStoreForConfig(config.Config{
		StorageBackend:           "minio",
		StorageS3Endpoint:        "http://localhost:9000",
		StorageS3Region:          "us-east-1",
		StorageS3Bucket:          "gogomail",
		StorageS3AccessKeyID:     "access",
		StorageS3SecretAccessKey: "secret",
	})
	if err != nil {
		t.Fatalf("objectStoreForConfig returned error: %v", err)
	}
	if _, ok := store.(*storage.S3Store); !ok {
		t.Fatalf("store = %T, want *storage.S3Store", store)
	}
}

func TestDriveServiceForConfigTreatsS3AndMinIOAsCompatibleLabels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		configBackend   string
		recordedBackend string
	}{
		{name: "s3 serves minio rows", configBackend: "s3", recordedBackend: "minio"},
		{name: "minio serves s3 rows", configBackend: "minio", recordedBackend: "s3"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			store := storage.NewLocalStore(t.TempDir())
			service := driveServiceForConfig(nil, config.Config{StorageBackend: tt.configBackend}, store)
			staged, err := service.StoreStagedObject(context.Background(), drive.StoreStagedObjectRequest{
				UserID:         "user-1",
				UploadID:       strings.ReplaceAll(tt.name, " ", "-"),
				StorageBackend: tt.recordedBackend,
				Body:           strings.NewReader("portable drive object"),
			})
			if err != nil {
				t.Fatalf("StoreStagedObject returned error: %v", err)
			}
			if staged.StorageBackend != tt.recordedBackend {
				t.Fatalf("storage backend = %q, want preserved recorded label %q", staged.StorageBackend, tt.recordedBackend)
			}
			if _, err := store.Stat(context.Background(), staged.StoragePath); err != nil {
				t.Fatalf("staged object not written through compatible store: %v", err)
			}
		})
	}
}

func TestDriveServiceForConfigTreatsLocalAndNFSAsCompatibleLabels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		configBackend   string
		recordedBackend string
	}{
		{name: "local serves nfs rows", configBackend: "local", recordedBackend: "nfs"},
		{name: "nfs serves local rows", configBackend: "nfs", recordedBackend: "local"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			store := storage.NewLocalStore(t.TempDir())
			service := driveServiceForConfig(nil, config.Config{StorageBackend: tt.configBackend}, store)
			staged, err := service.StoreStagedObject(context.Background(), drive.StoreStagedObjectRequest{
				UserID:         "user-1",
				UploadID:       strings.ReplaceAll(tt.name, " ", "-"),
				StorageBackend: tt.recordedBackend,
				Body:           strings.NewReader("portable nfs drive object"),
			})
			if err != nil {
				t.Fatalf("StoreStagedObject returned error: %v", err)
			}
			if staged.StorageBackend != tt.recordedBackend {
				t.Fatalf("storage backend = %q, want preserved recorded label %q", staged.StorageBackend, tt.recordedBackend)
			}
			if _, err := store.Stat(context.Background(), staged.StoragePath); err != nil {
				t.Fatalf("staged object not written through compatible store: %v", err)
			}
		})
	}
}

func TestStorageStoresForConfigAddsExplicitCompatLabels(t *testing.T) {
	t.Parallel()

	store := storage.NewLocalStore(t.TempDir())
	stores := storageStoresForConfig(config.Config{
		StorageBackend:             "s3",
		StorageBackendCompatLabels: []string{" local ", "NFS", "MINIO", "local"},
	}, store)
	for _, label := range []string{"s3", "minio", "local", "nfs"} {
		if stores[label] != store {
			t.Fatalf("stores[%q] = %T, want configured store", label, stores[label])
		}
	}
	if stores["swift"] != nil {
		t.Fatal("unexpected unsupported storage compatibility label")
	}
}

func TestStorageStoresForConfigDoesNotBridgeLocalWithoutCompatLabel(t *testing.T) {
	t.Parallel()

	store := storage.NewLocalStore(t.TempDir())
	stores := storageStoresForConfig(config.Config{StorageBackend: "s3"}, store)
	if stores["local"] != nil {
		t.Fatal("s3 backend should not serve local-labelled rows without explicit compatibility label")
	}
}

func TestS3OptionsForConfigPinsMinIOPathStyle(t *testing.T) {
	t.Parallel()

	opts, err := s3OptionsForConfig(config.Config{
		StorageS3Endpoint:        "http://localhost:9000",
		StorageS3Region:          "us-east-1",
		StorageS3Bucket:          "gogomail",
		StorageS3AccessKeyID:     "access",
		StorageS3SecretAccessKey: "secret",
	}, " minio ")
	if err != nil {
		t.Fatalf("s3OptionsForConfig returned error: %v", err)
	}
	if !opts.ForcePathStyle {
		t.Fatal("minio backend should force path-style requests")
	}
}

func TestS3OptionsForConfigPreservesS3PathStyleOverride(t *testing.T) {
	t.Parallel()

	opts, err := s3OptionsForConfig(config.Config{
		StorageS3Endpoint:        "https://s3.us-east-1.amazonaws.com",
		StorageS3Region:          "us-east-1",
		StorageS3Bucket:          "gogomail",
		StorageS3AccessKeyID:     "access",
		StorageS3SecretAccessKey: "secret",
	}, "s3")
	if err != nil {
		t.Fatalf("s3OptionsForConfig returned error: %v", err)
	}
	if opts.ForcePathStyle {
		t.Fatal("s3 backend should preserve virtual-hosted addressing by default")
	}

	opts, err = s3OptionsForConfig(config.Config{
		StorageS3Endpoint:        "https://s3.us-east-1.amazonaws.com",
		StorageS3Region:          "us-east-1",
		StorageS3Bucket:          "gogomail",
		StorageS3AccessKeyID:     "access",
		StorageS3SecretAccessKey: "secret",
		StorageS3ForcePathStyle:  true,
	}, "s3")
	if err != nil {
		t.Fatalf("s3OptionsForConfig with override returned error: %v", err)
	}
	if !opts.ForcePathStyle {
		t.Fatal("s3 backend should honor explicit path-style override")
	}
}

func TestS3OptionsForConfigWiresTLSOverrides(t *testing.T) {
	t.Parallel()

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()
	certFile := t.TempDir() + "/s3-ca.pem"
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
	if err := os.WriteFile(certFile, certPEM, 0o600); err != nil {
		t.Fatalf("write CA file: %v", err)
	}

	opts, err := s3OptionsForConfig(config.Config{
		StorageS3Endpoint:           "https://s3.internal.example",
		StorageS3Region:             "us-east-1",
		StorageS3Bucket:             "gogomail",
		StorageS3AccessKeyID:        "access",
		StorageS3SecretAccessKey:    "secret",
		StorageS3CACertFile:         certFile,
		StorageS3InsecureSkipVerify: true,
	}, "s3")
	if err != nil {
		t.Fatalf("s3OptionsForConfig returned error: %v", err)
	}
	if opts.HTTPClient == nil {
		t.Fatal("HTTPClient = nil, want custom S3 transport")
	}
	transport, ok := opts.HTTPClient.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("HTTPClient.Transport = %T, want *http.Transport", opts.HTTPClient.Transport)
	}
	if transport.TLSClientConfig == nil {
		t.Fatal("TLSClientConfig = nil, want S3 TLS overrides")
	}
	if !transport.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("InsecureSkipVerify = false, want configured override")
	}
	if transport.TLSClientConfig.MinVersion != tls.VersionTLS12 {
		t.Fatalf("MinVersion = %x, want TLS 1.2", transport.TLSClientConfig.MinVersion)
	}
	if len(transport.TLSClientConfig.RootCAs.Subjects()) == 0 {
		t.Fatal("RootCAs is empty, want custom CA bundle appended")
	}
}

func TestDKIMKeyProviderMapsRepositoryKey(t *testing.T) {
	t.Parallel()

	provider := dkimKeyProvider{repository: fakeDKIMKeyRepository{key: maildb.DKIMKey{
		DomainID:      "domain-1",
		DomainName:    "example.com",
		Selector:      "s1",
		PrivateKeyPEM: "private",
	}}}
	key, err := provider.DKIMKey(context.Background(), delivery.Job{
		QueuedMessage: delivery.QueuedMessage{DomainID: "domain-1"},
	})
	if err != nil {
		t.Fatalf("DKIMKey returned error: %v", err)
	}
	if key.Domain != "example.com" || key.Selector != "s1" || key.PrivateKeyPEM != "private" {
		t.Fatalf("key = %+v", key)
	}
}

func TestNewIMAPGatewayRuntimeWiresMailboxEventBroker(t *testing.T) {
	t.Parallel()

	runtime := newIMAPGatewayRuntime(nil, nil, nil, nil)
	if runtime.service == nil {
		t.Fatal("service is nil")
	}
	if runtime.events == nil {
		t.Fatal("mailbox event broker is nil")
	}
	if _, err := runtime.store.ListMailboxes(context.Background(), imapgw.ListMailboxesRequest{UserID: "user-1"}); err == nil || !strings.Contains(err.Error(), "imap mailbox repository is required") {
		t.Fatalf("store adapter was not wired through service boundary: %v", err)
	}
	if _, err := runtime.backend.Authenticate(context.Background(), "user@example.com", "secret"); err == nil || !strings.Contains(err.Error(), "imap authenticator is required") {
		t.Fatalf("backend authenticator was not wired through IMAP boundary: %v", err)
	}

	events, cancel, err := runtime.service.SubscribeIMAPMailbox(context.Background(), "user-1", "inbox")
	if err != nil {
		t.Fatalf("SubscribeIMAPMailbox returned error: %v", err)
	}
	defer cancel()

	if err := runtime.events.Publish(context.Background(), imapgw.MailboxEvent{
		Type:      imapgw.MailboxEventExists,
		UserID:    "user-1",
		MailboxID: "inbox",
		Messages:  1,
	}); err != nil {
		t.Fatalf("Publish returned error: %v", err)
	}

	select {
	case event := <-events:
		if event.Type != imapgw.MailboxEventExists || event.Messages != 1 {
			t.Fatalf("event = %+v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for mailbox event")
	}
}

func TestNewIMAPMailboxEventRouterPublishesStoredMailToBroker(t *testing.T) {
	t.Parallel()

	broker := imapgw.NewMailboxEventBroker(1)
	events, cancel, err := broker.Subscribe(context.Background(), "user-1", "inbox")
	if err != nil {
		t.Fatalf("Subscribe returned error: %v", err)
	}
	defer cancel()

	router, err := newIMAPMailboxEventRouter(fakeIMAPUIDEnsurer{}, broker, nil)
	if err != nil {
		t.Fatalf("newIMAPMailboxEventRouter returned error: %v", err)
	}
	err = router.HandleEvent(context.Background(), eventstream.Message{Payload: []byte(`{
		"event":"mail.stored",
		"schema_version":"2026-05-04.mail-stored.v1",
		"message_id":"msg-1",
		"user_id":"user-1",
		"folder_id":"inbox"
	}`)})
	if err != nil {
		t.Fatalf("HandleEvent returned error: %v", err)
	}

	select {
	case event := <-events:
		if event.Type != imapgw.MailboxEventExists || event.UserID != "user-1" || event.MailboxID != "inbox" || event.UID != 42 {
			t.Fatalf("event = %+v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for IMAP mailbox event")
	}
}

func TestIMAPServerOptionsForConfigUsesRuntimeBackend(t *testing.T) {
	t.Parallel()

	runtime := newIMAPGatewayRuntime(nil, nil, nil, nil)
	opts, err := imapServerOptionsForConfig(config.Config{
		IMAPAddr:              " :1143 ",
		IMAPAllowInsecureAuth: true,
		IMAPMaxConnections:    256,
		IMAPReadTimeout:       45 * time.Second,
		IMAPWriteTimeout:      15 * time.Second,
		IMAPIdleTimeout:       35 * time.Minute,
		SMTPDomain:            "localhost",
	}, runtime.backend)
	if err != nil {
		t.Fatalf("imapServerOptionsForConfig returned error: %v", err)
	}
	if opts.Addr != ":1143" || opts.Backend == nil || opts.TLSConfig != nil || !opts.AllowInsecureAuth || opts.MaxConnections != 256 || opts.ReadTimeout != 45*time.Second || opts.WriteTimeout != 15*time.Second || opts.IdleTimeout != 35*time.Minute {
		t.Fatalf("options = %+v", opts)
	}
}

func TestPOP3ServerForConfigUsesMaxConnections(t *testing.T) {
	t.Parallel()

	server, err := pop3ServerForConfig(config.Config{
		POP3MaxConnections: 96,
		POP3IdleTimeout:    3 * time.Minute,
	}, nil, nil)
	if err != nil {
		t.Fatalf("pop3ServerForConfig returned error: %v", err)
	}
	if server.MaxConnections != 96 {
		t.Fatalf("MaxConnections = %d, want 96", server.MaxConnections)
	}
	if server.IdleTimeout != 3*time.Minute {
		t.Fatalf("IdleTimeout = %s, want 3m", server.IdleTimeout)
	}
}

type fakeIMAPUIDEnsurer struct{}

func (fakeIMAPUIDEnsurer) EnsureIMAPMessageUID(_ context.Context, userID string, mailboxID string, messageID string) (maildb.IMAPMessageUID, error) {
	return maildb.IMAPMessageUID{
		MessageID: imapgw.MessageID(messageID),
		MailboxID: imapgw.MailboxID(mailboxID),
		UID:       imapgw.UID(42),
		ModSeq:    1,
	}, nil
}

func TestNewIMAPServerBuildsProtocolShell(t *testing.T) {
	t.Parallel()

	runtime := newIMAPGatewayRuntime(nil, nil, nil, nil)
	opts, err := imapServerOptionsForConfig(config.Config{IMAPAddr: ":1143", IMAPAllowInsecureAuth: true}, runtime.backend)
	if err != nil {
		t.Fatalf("imapServerOptionsForConfig returned error: %v", err)
	}
	server, err := newIMAPServer(opts)
	if err != nil {
		t.Fatalf("newIMAPServer returned error: %v", err)
	}
	if server == nil || server.Options().Addr != ":1143" {
		t.Fatalf("server = %+v", server)
	}
}

func TestAllInOneHTTPModeIncludesMailAndAdminAPIs(t *testing.T) {
	t.Parallel()

	if !modeIncludesMailAPI(ModeAllInOne) {
		t.Fatal("all-in-one did not include Mail API routes")
	}
	if !modeIncludesAdminAPI(ModeAllInOne) {
		t.Fatal("all-in-one did not include Admin API routes")
	}
	if modeIncludesAdminAPI(ModeMailAPI) {
		t.Fatal("mail-api should not include Admin API routes")
	}
	if modeIncludesMailAPI(ModeAdminAPI) {
		t.Fatal("admin-api should not include Mail API routes")
	}
}

func TestSubmissionServerOptionsSelectSMTPSAddress(t *testing.T) {
	t.Parallel()

	cfg := config.Load()
	cfg.SMTPDomain = "mail.example"
	cfg.SubmissionAddr = ":2587"
	cfg.SubmissionSMTPSAddr = " :2465 "
	cfg.SMTPReadTimeout = 7 * time.Second
	cfg.SMTPWriteTimeout = 8 * time.Second
	cfg.SubmissionMaxConnections = 64
	cfg.SubmissionMaxMessageBytes = 1234
	cfg.SubmissionMaxRecipients = 12
	cfg.SubmissionAllowInsecureAuth = false
	cfg.SubmissionSupportDSN = true
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	backend := gosmtp.BackendFunc(func(*gosmtp.Conn) (gosmtp.Session, error) {
		return nil, nil
	})

	opts := submissionServerOptions(cfg, nil, backend, tlsConfig, true)

	if opts.Addr != ":2465" {
		t.Fatalf("Addr = %q, want SMTPS addr", opts.Addr)
	}
	if !opts.ImplicitTLS {
		t.Fatal("ImplicitTLS = false, want true")
	}
	if opts.TLSConfig != tlsConfig {
		t.Fatal("TLSConfig was not preserved")
	}
	if opts.AllowInsecureAuth {
		t.Fatal("AllowInsecureAuth = true, want false")
	}
	if opts.MaxConnections != 64 {
		t.Fatalf("MaxConnections = %d, want 64", opts.MaxConnections)
	}
	if !opts.EnableDSN {
		t.Fatal("EnableDSN = false, want true")
	}
}

func TestAttachmentScanHooksForConfigDisabledByDefault(t *testing.T) {
	t.Parallel()

	hooks, err := attachmentScanHooksForConfig(config.Config{AttachmentScanBackend: "none"}, nil, "test")
	if err != nil {
		t.Fatalf("attachmentScanHooksForConfig returned error: %v", err)
	}
	if len(hooks) != 0 {
		t.Fatalf("hooks = %d, want none", len(hooks))
	}
}

func TestAttachmentScanHooksForConfigWebhook(t *testing.T) {
	t.Parallel()

	hooks, err := attachmentScanHooksForConfig(config.Config{
		AttachmentScanBackend:    "webhook",
		AttachmentScanWebhookURL: "http://scanner.example.test/scan",
		AttachmentScanTimeout:    time.Second,
	}, nil, "test")
	if err != nil {
		t.Fatalf("attachmentScanHooksForConfig returned error: %v", err)
	}
	if len(hooks) != 1 {
		t.Fatalf("hooks = %d, want one webhook scanner hook", len(hooks))
	}
}

func TestPushNotificationSinkForConfigWebhook(t *testing.T) {
	t.Parallel()

	sink, err := pushNotificationSinkForConfig(config.Config{
		PushNotifyBackend:        "webhook",
		PushNotifyWebhookURL:     "http://push.example.test/send",
		PushNotifyWebhookTimeout: time.Second,
	}, nil)
	if err != nil {
		t.Fatalf("pushNotificationSinkForConfig returned error: %v", err)
	}
	if sink == nil {
		t.Fatal("sink is nil")
	}
}

func TestPushNotificationSinkForConfigRejectsUnknownBackend(t *testing.T) {
	t.Parallel()

	if _, err := pushNotificationSinkForConfig(config.Config{PushNotifyBackend: "direct"}, nil); err == nil {
		t.Fatal("pushNotificationSinkForConfig accepted unknown backend")
	}
}

func TestAPIMeteringHandlerDefaultsToOriginalHandler(t *testing.T) {
	t.Parallel()

	next := &sentinelHTTPHandler{}
	handler := apiMeteringHandler(next, config.Config{APIMeteringBackend: "none"}, nil, nil, nil, "")
	if handler != next {
		t.Fatal("apiMeteringHandler wrapped handler when backend is none")
	}
}

func TestAPIUsageExportManifestSignerConfig(t *testing.T) {
	disabled := config.Config{APIUsageExportManifestSignerBackend: "disabled"}
	if signer := apiUsageExportManifestSigner(disabled); signer != nil {
		t.Fatalf("disabled signer = %#v", signer)
	}
	if verifier := apiUsageExportManifestVerifier(disabled); verifier != nil {
		t.Fatalf("disabled verifier = %#v", verifier)
	}

	enabled := config.Config{
		APIUsageExportManifestSignerBackend: "local-hmac",
		APIUsageExportManifestSignerKeyID:   "key-1",
		APIUsageExportManifestSignerSecret:  "secret",
	}
	if signer := apiUsageExportManifestSigner(enabled); signer == nil {
		t.Fatal("local-hmac signer is nil")
	}
	if verifier := apiUsageExportManifestVerifier(enabled); verifier == nil {
		t.Fatal("local-hmac verifier is nil")
	}

	privateKey := ed25519.NewKeyFromSeed([]byte(strings.Repeat("s", ed25519.SeedSize)))
	publicKey := privateKey.Public().(ed25519.PublicKey)
	ed25519Enabled := config.Config{
		APIUsageExportManifestSignerBackend: "local-ed25519",
		APIUsageExportManifestSignerKeyID:   "key-2",
		APIUsageExportSignerPrivateKey:      base64.StdEncoding.EncodeToString(privateKey),
		APIUsageExportSignerPublicKey:       base64.StdEncoding.EncodeToString(publicKey),
	}
	if signer := apiUsageExportManifestSigner(ed25519Enabled); signer == nil {
		t.Fatal("local-ed25519 signer is nil")
	}
	if verifier := apiUsageExportManifestVerifier(ed25519Enabled); verifier == nil {
		t.Fatal("local-ed25519 verifier is nil")
	}

	remoteEd25519Enabled := config.Config{
		APIUsageExportManifestSignerBackend: "remote-ed25519",
		APIUsageExportManifestSignerKeyID:   "key-3",
		APIUsageExportSignerURL:             "https://signer.example.test/sign",
		APIUsageExportSignerPublicKey:       base64.StdEncoding.EncodeToString(publicKey),
	}
	if signer := apiUsageExportManifestSigner(remoteEd25519Enabled); signer == nil {
		t.Fatal("remote-ed25519 signer is nil")
	}
	if verifier := apiUsageExportManifestVerifier(remoteEd25519Enabled); verifier == nil {
		t.Fatal("remote-ed25519 verifier is nil")
	}
}

func TestAPIUsageExportManifestSignerRejectsOversizedKeysBeforeDecode(t *testing.T) {
	t.Parallel()

	oversizedPrivate := config.Config{
		APIUsageExportManifestSignerBackend: "local-ed25519",
		APIUsageExportManifestSignerKeyID:   "key-1",
		APIUsageExportSignerPrivateKey:      strings.Repeat("a", base64.StdEncoding.EncodedLen(ed25519.PrivateKeySize)+1),
		APIUsageExportSignerPublicKey:       base64.StdEncoding.EncodeToString([]byte(strings.Repeat("p", ed25519.PublicKeySize))),
	}
	if signer := apiUsageExportManifestSigner(oversizedPrivate); signer != nil {
		t.Fatal("apiUsageExportManifestSigner accepted oversized private key")
	}

	oversizedPublic := config.Config{
		APIUsageExportManifestSignerBackend: "remote-ed25519",
		APIUsageExportManifestSignerKeyID:   "key-2",
		APIUsageExportSignerURL:             "https://signer.example.test/sign",
		APIUsageExportSignerPublicKey:       strings.Repeat("a", base64.StdEncoding.EncodedLen(ed25519.PublicKeySize)+1),
	}
	if signer := apiUsageExportManifestSigner(oversizedPublic); signer != nil {
		t.Fatal("apiUsageExportManifestSigner accepted oversized public key")
	}
	if verifier := apiUsageExportManifestVerifier(oversizedPublic); verifier != nil {
		t.Fatal("apiUsageExportManifestVerifier accepted oversized public key")
	}
}

func TestAPIMeteringHandlerWrapsSlogBackend(t *testing.T) {
	t.Parallel()

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})
	handler := apiMeteringHandler(next, config.Config{
		APIMeteringBackend: "slog",
		APIMeteringTimeout: 100 * time.Millisecond,
	}, nil, nil, nil, "")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/info", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusAccepted)
	}
}

func TestAPIMeteringHandlerRequiresOutboxDB(t *testing.T) {
	t.Parallel()

	next := &sentinelHTTPHandler{}
	handler := apiMeteringHandler(next, config.Config{APIMeteringBackend: "outbox"}, nil, nil, nil, "")
	if handler != next {
		t.Fatal("apiMeteringHandler wrapped outbox backend without database handle")
	}
}

func TestMeteringIdentityResolverUsesJWTClaims(t *testing.T) {
	t.Parallel()

	manager, err := auth.NewTokenManager("secret")
	if err != nil {
		t.Fatalf("NewTokenManager returned error: %v", err)
	}
	token, err := manager.Sign(auth.Claims{UserID: "user-1", DomainID: "domain-1"}, time.Minute)
	if err != nil {
		t.Fatalf("Sign returned error: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/messages", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	id := meteringIdentityResolver(manager, "")(req)
	if id.UserID != "user-1" || id.DomainID != "domain-1" {
		t.Fatalf("identity = %+v", id)
	}
	if id.AuthSource != apimeter.AuthSourceBearer || id.PrincipalID != "user-1" {
		t.Fatalf("identity principal = %+v", id)
	}
}

func TestTokenManagerForConfigAttachesRevocationChecker(t *testing.T) {
	t.Parallel()

	checker := &staticSessionVersionChecker{version: 5}
	manager, err := tokenManagerForConfig(config.Config{AuthJWTSecret: "admin-secret"}, checker)
	if err != nil {
		t.Fatalf("tokenManagerForConfig returned error: %v", err)
	}
	if manager == nil {
		t.Fatal("tokenManagerForConfig returned nil manager")
	}
	token, err := manager.Sign(auth.Claims{UserID: "user-1", DomainID: "domain-1", Role: "company_admin", SessionVersion: 4}, time.Minute)
	if err != nil {
		t.Fatalf("Sign returned error: %v", err)
	}
	if _, err := manager.VerifyFull(context.Background(), token); err == nil {
		t.Fatal("VerifyFull accepted token below configured session version")
	}
}

func TestTokenManagerForConfigAllowsMissingSecret(t *testing.T) {
	t.Parallel()

	manager, err := tokenManagerForConfig(config.Config{}, nil)
	if err != nil {
		t.Fatalf("tokenManagerForConfig returned error: %v", err)
	}
	if manager != nil {
		t.Fatal("tokenManagerForConfig returned manager for empty secret")
	}
}

func TestMeteringIdentityResolverUsesAdminToken(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/api-usage/daily", nil)
	req.Header.Set("X-Admin-Token", "secret")

	id := meteringIdentityResolver(nil, "secret")(req)
	if id.AuthSource != apimeter.AuthSourceAdminToken || id.PrincipalID != apimeter.AuthSourceAdminToken {
		t.Fatalf("identity = %+v", id)
	}
}

func TestMeteringIdentityResolverUsesAPIKeyContext(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/messages", nil)
	req.Header.Set("X-Gogomail-User-ID", "user-1")
	req = req.WithContext(apikeys.ContextWithKeyInfo(req.Context(), &apikeys.KeyInfo{
		ID:       "key-1",
		DomainID: "domain-1",
		Scopes:   []string{"mail:read"},
	}))

	id := meteringIdentityResolver(nil, "")(req)
	if id.AuthSource != apimeter.AuthSourceAPIKey || id.APIKeyID != "key-1" || id.DomainID != "domain-1" || id.UserID != "user-1" {
		t.Fatalf("identity = %+v", id)
	}
	if id.PrincipalID != "user-1" {
		t.Fatalf("principal = %q, want user-1", id.PrincipalID)
	}
}

func TestMeteringIdentityResolverUsesResolvedAPIKeyUser(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/messages?user_email=user%40example.com", nil)
	req.Header.Set("X-Gogomail-Resolved-User-ID", "user-1")
	req = req.WithContext(apikeys.ContextWithKeyInfo(req.Context(), &apikeys.KeyInfo{
		ID:       "key-1",
		DomainID: "domain-1",
		Scopes:   []string{"mail:read"},
	}))

	id := meteringIdentityResolver(nil, "")(req)
	if id.AuthSource != apimeter.AuthSourceAPIKey || id.UserID != "user-1" || id.PrincipalID != "user-1" {
		t.Fatalf("identity = %+v", id)
	}
}

func TestMeteringAdminTokenMatchesTrimsAndRejectsMismatches(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/api-usage/daily", nil)
	req.Header.Set("X-Admin-Token", " secret ")
	if !meteringAdminTokenMatches(req, "secret") {
		t.Fatal("meteringAdminTokenMatches rejected matching trimmed token")
	}

	for _, got := range []string{"", "secre", "secret-longer"} {
		req := httptest.NewRequest(http.MethodGet, "/admin/v1/api-usage/daily", nil)
		req.Header.Set("X-Admin-Token", got)
		if meteringAdminTokenMatches(req, "secret") {
			t.Fatalf("meteringAdminTokenMatches(%q) = true, want false", got)
		}
	}
}

type fakeDKIMKeyRepository struct {
	key          maildb.DKIMKey
	lastDomainID string
}

type sentinelHTTPHandler struct{}

func (*sentinelHTTPHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

type staticSessionVersionChecker struct {
	version int64
}

func (s *staticSessionVersionChecker) SessionVersionFor(context.Context, string) (int64, error) {
	return s.version, nil
}

func (r fakeDKIMKeyRepository) ActiveDKIMKey(_ context.Context, domainID string) (maildb.DKIMKey, error) {
	r.lastDomainID = domainID
	return r.key, nil
}
