package app

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"

	"github.com/gogomail/gogomail/internal/config"
	"github.com/gogomail/gogomail/internal/directory"
	"github.com/gogomail/gogomail/internal/ldapgw"
	"github.com/gogomail/gogomail/internal/maildb"
)

func runLDAPGateway(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	db, err := openDatabase(ctx, cfg)
	if err != nil {
		return err
	}
	defer db.Close()

	auth := maildb.NewRepository(db)
	querier := &ldapDirectoryQuerier{
		repo:       directory.NewRepository(db),
		companyID:  cfg.LDAPCompanyID,
		baseDomain: cfg.LDAPBaseDomain,
	}
	tlsConfig, err := ldapTLSConfig(cfg)
	if err != nil {
		return err
	}
	namingContexts := []string{}
	if strings.TrimSpace(cfg.LDAPBaseDomain) != "" {
		namingContexts = append(namingContexts, strings.TrimSpace(cfg.LDAPBaseDomain))
	}

	errCh := make(chan error, 2)
	var servers []*ldapgw.LDAPServer
	if addr := strings.TrimSpace(cfg.LDAPAddr); addr != "" {
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			return err
		}
		srv := ldapgw.NewServerWithOptions(ln, auth, querier, ldapgw.ServerOptions{
			TLSConfig:      tlsConfig,
			NamingContexts: namingContexts,
			ReferralURLs:   cfg.LDAPReferralURLs,
			Metrics:        ldapMetrics(cfg, logger),
		})
		servers = append(servers, srv)
		go func() {
			logger.Info("ldap gateway listening", "mode", ModeLDAPGateway, "addr", ln.Addr().String(), "starttls_configured", tlsConfig != nil)
			errCh <- srv.Serve()
		}()
	}
	if addr := strings.TrimSpace(cfg.LDAPSAddr); addr != "" {
		if tlsConfig == nil {
			return errors.New("GOGOMAIL_LDAPS_ADDR requires LDAP TLS certificate and key files")
		}
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			return err
		}
		tlsLn := tls.NewListener(ln, tlsConfig)
		srv := ldapgw.NewServerWithOptions(tlsLn, auth, querier, ldapgw.ServerOptions{
			NamingContexts: namingContexts,
			ReferralURLs:   cfg.LDAPReferralURLs,
			Metrics:        ldapMetrics(cfg, logger),
		})
		servers = append(servers, srv)
		go func() {
			logger.Info("ldaps gateway listening", "mode", ModeLDAPGateway, "addr", ln.Addr().String())
			errCh <- srv.Serve()
		}()
	}
	if len(servers) == 0 {
		return errors.New("at least one LDAP listener address must be configured")
	}

	select {
	case <-ctx.Done():
		for _, srv := range servers {
			if err := srv.Close(); err != nil {
				logger.Warn("close ldap server", "error", err)
			}
		}
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}

func ldapTLSConfig(cfg config.Config) (*tls.Config, error) {
	if cfg.LDAPTLSCertFile == "" && cfg.LDAPTLSKeyFile == "" {
		return nil, nil
	}
	if cfg.LDAPTLSCertFile == "" || cfg.LDAPTLSKeyFile == "" {
		return nil, errors.New("both LDAP TLS certificate and key files are required")
	}
	cert, err := tls.LoadX509KeyPair(cfg.LDAPTLSCertFile, cfg.LDAPTLSKeyFile)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}, nil
}

type ldapDirectoryQuerier struct {
	repo       *directory.Repository
	companyID  string
	baseDomain string
}

func (q *ldapDirectoryQuerier) SearchPrincipals(ctx context.Context, req ldapgw.DirectorySearchRequest) ([]ldapgw.PrincipalEntry, error) {
	query := ldapFilterToQuery(req.Filter)
	limit := req.Limit
	if limit <= 0 {
		limit = 100
	}
	baseDN := q.baseDomain
	if baseDN == "" {
		baseDN = "dc=local"
	}
	if req.Scope == 0 {
		if kind, id, ok := ldapPrincipalFromDN(req.BaseDN); ok {
			principal, err := q.repo.ResolvePrincipal(ctx, directory.ResolvePrincipalRequest{
				ID:         id,
				Kind:       kind,
				ActiveOnly: true,
			})
			if err != nil {
				return nil, nil
			}
			entry, err := q.ldapPrincipalEntry(ctx, principal, baseDN, req.Attrs)
			if err != nil {
				return nil, err
			}
			return []ldapgw.PrincipalEntry{entry}, nil
		}
	}
	principals, err := q.repo.SearchPrincipals(ctx, directory.SearchPrincipalsRequest{
		CompanyID:  q.companyID,
		Kinds:      req.Kinds,
		Query:      query,
		ActiveOnly: true,
		Limit:      limit,
		Offset:     req.Offset,
	})
	if err != nil {
		return nil, err
	}
	entries := make([]ldapgw.PrincipalEntry, 0, len(principals))
	expandMembers := ldapShouldExpandGroupMembers(req.Attrs)
	expandMemberOf := ldapShouldExpandMemberOf(req.Attrs)
	var membershipsByGroup map[string][]directory.GroupMembership
	var membershipsByMember map[directory.PrincipalRef][]directory.GroupMembership
	if expandMembers {
		groupIDs := make([]string, 0, len(principals))
		for _, p := range principals {
			if p.Kind == directory.PrincipalKindGroup {
				groupIDs = append(groupIDs, p.ID)
			}
		}
		membershipsByGroup, err = q.repo.ListGroupMembershipsForGroups(ctx, q.companyID, groupIDs, true, directory.MaxGroupMembershipListLimit)
		if err != nil {
			return nil, err
		}
	}
	if expandMemberOf {
		members := make([]directory.PrincipalRef, 0, len(principals))
		for _, p := range principals {
			members = append(members, directory.PrincipalRef{Kind: p.Kind, ID: p.ID})
		}
		membershipsByMember, err = q.repo.ListGroupMembershipsForMembers(ctx, q.companyID, members, true, directory.MaxGroupMembershipListLimit)
		if err != nil {
			return nil, err
		}
	}
	for _, p := range principals {
		entry := ldapPrincipalEntry(p, baseDN)
		if expandMembers && p.Kind == directory.PrincipalKindGroup {
			for _, membership := range membershipsByGroup[p.ID] {
				if memberDN := ldapPrincipalKindIDDN(membership.MemberKind, membership.MemberID, baseDN); memberDN != "" {
					entry.Members = append(entry.Members, memberDN)
				}
			}
		}
		if expandMemberOf {
			for _, membership := range membershipsByMember[directory.PrincipalRef{Kind: p.Kind, ID: p.ID}] {
				if groupDN := ldapPrincipalKindIDDN(directory.PrincipalKindGroup, membership.GroupID, baseDN); groupDN != "" {
					entry.MemberOf = append(entry.MemberOf, groupDN)
				}
			}
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func (q *ldapDirectoryQuerier) ldapPrincipalEntry(ctx context.Context, p directory.Principal, baseDN string, attrs []string) (ldapgw.PrincipalEntry, error) {
	entry := ldapPrincipalEntry(p, baseDN)
	if p.Kind == directory.PrincipalKindGroup && ldapShouldExpandGroupMembers(attrs) {
		memberships, err := q.repo.ListGroupMemberships(ctx, directory.ListGroupMembershipsRequest{
			CompanyID:  q.companyID,
			GroupID:    p.ID,
			ActiveOnly: true,
			Limit:      directory.MaxGroupMembershipListLimit,
		})
		if err != nil {
			return ldapgw.PrincipalEntry{}, err
		}
		for _, membership := range memberships {
			if memberDN := ldapPrincipalKindIDDN(membership.MemberKind, membership.MemberID, baseDN); memberDN != "" {
				entry.Members = append(entry.Members, memberDN)
			}
		}
	}
	if ldapShouldExpandMemberOf(attrs) {
		memberships, err := q.repo.ListGroupMemberships(ctx, directory.ListGroupMembershipsRequest{
			CompanyID:  q.companyID,
			MemberKind: p.Kind,
			MemberID:   p.ID,
			ActiveOnly: true,
			Limit:      directory.MaxGroupMembershipListLimit,
		})
		if err != nil {
			return ldapgw.PrincipalEntry{}, err
		}
		for _, membership := range memberships {
			if groupDN := ldapPrincipalKindIDDN(directory.PrincipalKindGroup, membership.GroupID, baseDN); groupDN != "" {
				entry.MemberOf = append(entry.MemberOf, groupDN)
			}
		}
	}
	return entry, nil
}

func ldapShouldExpandGroupMembers(attrs []string) bool {
	return ldapAttributeRequested(attrs, "member")
}

func ldapShouldExpandMemberOf(attrs []string) bool {
	return ldapAttributeRequested(attrs, "memberOf")
}

func ldapAttributeRequested(attrs []string, target string) bool {
	if len(attrs) == 0 {
		return true
	}
	for _, attr := range attrs {
		switch strings.ToLower(strings.TrimSpace(attr)) {
		case strings.ToLower(target), "*":
			return true
		case "1.1", "+":
			continue
		}
	}
	return false
}

func ldapPrincipalEntry(p directory.Principal, baseDN string) ldapgw.PrincipalEntry {
	return ldapgw.PrincipalEntry{
		DN:           ldapPrincipalDN(p, baseDN),
		Kind:         p.Kind,
		CN:           p.DisplayName,
		Mail:         p.PrimaryEmail,
		UID:          p.ID,
		OU:           p.DisplayName,
		DisplayName:  p.DisplayName,
		ResourceType: p.ResourceType,
	}
}

func ldapPrincipalKindIDDN(kind, id, baseDN string) string {
	return ldapPrincipalDN(directory.Principal{ID: id, Kind: kind}, baseDN)
}

func ldapPrincipalDN(p directory.Principal, baseDN string) string {
	id := ldapEscapeDNValue(p.ID)
	switch p.Kind {
	case directory.PrincipalKindOrganization:
		return fmt.Sprintf("ou=%s,ou=organizations,%s", id, baseDN)
	case directory.PrincipalKindGroup:
		return fmt.Sprintf("cn=%s,ou=groups,%s", id, baseDN)
	case directory.PrincipalKindResource:
		return fmt.Sprintf("cn=%s,ou=resources,%s", id, baseDN)
	default:
		return fmt.Sprintf("uid=%s,ou=users,%s", id, baseDN)
	}
}

func ldapPrincipalFromDN(dn string) (kind string, id string, ok bool) {
	parts := ldapSplitDN(dn)
	if len(parts) < 2 {
		return "", "", false
	}
	first := strings.SplitN(strings.TrimSpace(parts[0]), "=", 2)
	if len(first) != 2 {
		return "", "", false
	}
	attr := strings.ToLower(strings.TrimSpace(first[0]))
	value := ldapUnescapeDNValue(strings.TrimSpace(first[1]))
	parent := strings.ToLower(strings.TrimSpace(parts[1]))
	switch {
	case attr == "uid" && parent == "ou=users":
		return directory.PrincipalKindUser, value, true
	case attr == "ou" && parent == "ou=organizations":
		return directory.PrincipalKindOrganization, value, true
	case attr == "cn" && parent == "ou=groups":
		return directory.PrincipalKindGroup, value, true
	case attr == "cn" && parent == "ou=resources":
		return directory.PrincipalKindResource, value, true
	default:
		return "", "", false
	}
}

func ldapEscapeDNValue(value string) string {
	var b strings.Builder
	for i := 0; i < len(value); i++ {
		c := value[i]
		needsEscape := c == ',' || c == '+' || c == '"' || c == '\\' || c == '<' || c == '>' || c == ';' || c == '=' || c == 0
		if i == 0 && (c == ' ' || c == '#') {
			needsEscape = true
		}
		if i == len(value)-1 && c == ' ' {
			needsEscape = true
		}
		if needsEscape {
			b.WriteString(fmt.Sprintf("\\%02x", c))
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

func ldapSplitDN(dn string) []string {
	var parts []string
	var b strings.Builder
	escaped := false
	for _, r := range strings.TrimSpace(dn) {
		if escaped {
			b.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			b.WriteRune(r)
			escaped = true
			continue
		}
		if r == ',' {
			parts = append(parts, strings.TrimSpace(b.String()))
			b.Reset()
			continue
		}
		b.WriteRune(r)
	}
	parts = append(parts, strings.TrimSpace(b.String()))
	return parts
}

func ldapUnescapeDNValue(value string) string {
	var b strings.Builder
	for i := 0; i < len(value); i++ {
		if value[i] != '\\' || i+1 >= len(value) {
			b.WriteByte(value[i])
			continue
		}
		if i+2 < len(value) && isHex(value[i+1]) && isHex(value[i+2]) {
			b.WriteByte(fromHex(value[i+1])<<4 | fromHex(value[i+2]))
			i += 2
			continue
		}
		i++
		b.WriteByte(value[i])
	}
	return b.String()
}

func isHex(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

func fromHex(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	default:
		return c - 'A' + 10
	}
}

func ldapFilterToQuery(filter string) string {
	filter = strings.TrimSpace(filter)
	filter = strings.TrimPrefix(filter, "(")
	filter = strings.TrimSuffix(filter, ")")
	if idx := strings.Index(filter, "="); idx >= 0 {
		attr := strings.ToLower(strings.TrimSpace(filter[:idx]))
		switch attr {
		case "cn", "mail", "uid", "displayname", "givenname", "sn", "ou", "description", "name", "canonicalname", "samaccountname", "userprincipalname", "mailnickname", "proxyaddresses":
		default:
			return ""
		}
		val := filter[idx+1:]
		val = strings.Trim(val, "*")
		if attr == "canonicalname" {
			if idx := strings.LastIndex(val, "/"); idx >= 0 {
				val = val[idx+1:]
			}
		}
		return val
	}
	return ""
}
