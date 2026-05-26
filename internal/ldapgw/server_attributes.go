package ldapgw

import (
	"crypto/sha1"
	"fmt"
	"strings"
	"time"
)

func principalLDAPAttributes(p PrincipalEntry) map[string][]string {
	kind := strings.ToLower(strings.TrimSpace(p.Kind))
	if kind == "" {
		kind = "user"
	}
	cn := firstNonEmpty(p.CN, p.DisplayName, p.UID)
	objectCategory := ldapObjectCategory(kind)
	attrs := map[string][]string{
		"cn":                {cn},
		"name":              {cn},
		"uid":               {p.UID},
		"displayName":       {firstNonEmpty(p.DisplayName, cn)},
		"canonicalName":     nonEmptyLDAPValues([]string{ldapCanonicalName(p.DN)}),
		"distinguishedName": nonEmptyLDAPValues([]string{p.DN}),
		"instanceType":      {"4"},
		"objectCategory":    {objectCategory},
		"objectGUID":        {string(ldapEntryUUIDBytes(p.DN))},
		"mailNickname":      nonEmptyLDAPValues([]string{p.UID}),
		"proxyAddresses":    ldapProxyAddresses(p.Mail),
		"sAMAccountName":    {p.UID},
		"userPrincipalName": nonEmptyLDAPValues([]string{p.Mail}),
		"whenCreated":       {"19700101000000.0Z"},
		"whenChanged":       {"19700101000000.0Z"},
		"uSNCreated":        {ldapUpdateSequenceNumber(p.DN)},
		"uSNChanged":        {ldapUpdateSequenceNumber(p.DN)},
	}
	if len(attrs["canonicalName"]) == 0 {
		delete(attrs, "canonicalName")
	}
	if len(attrs["distinguishedName"]) == 0 {
		delete(attrs, "distinguishedName")
	}
	if sid := ldapObjectSIDBytes(p.DN, kind); len(sid) > 0 {
		attrs["objectSid"] = []string{string(sid)}
	}
	if len(attrs["userPrincipalName"]) == 0 {
		delete(attrs, "userPrincipalName")
	}
	if memberOf := nonEmptyLDAPValues(p.MemberOf); len(memberOf) > 0 {
		attrs["memberOf"] = memberOf
	}
	switch kind {
	case "organization":
		attrs["objectClass"] = []string{"top", "organizationalUnit"}
		attrs["ou"] = []string{firstNonEmpty(p.OU, p.DisplayName, cn)}
	case "group":
		attrs["objectClass"] = []string{"top", "groupOfNames"}
		members := nonEmptyLDAPValues(p.Members)
		if len(members) == 0 {
			members = []string{ldapEmptyGroupMemberFallbackDN(p, cn)}
		}
		attrs["member"] = members
	case "resource":
		attrs["objectClass"] = []string{"top", "device"}
		if p.ResourceType != "" {
			attrs["description"] = []string{p.ResourceType}
		}
	default:
		attrs["objectClass"] = []string{"top", "person", "organizationalPerson", "inetOrgPerson", "user"}
		attrs["accountExpires"] = []string{"9223372036854775807"}
		attrs["primaryGroupID"] = []string{"513"}
		attrs["userAccountControl"] = []string{"512"}
		if p.Mail != "" {
			attrs["mail"] = []string{p.Mail}
		}
		if p.GivenName != "" {
			attrs["givenName"] = []string{p.GivenName}
		}
		attrs["sn"] = []string{firstNonEmpty(p.SN, p.DisplayName, cn)}
	}
	for k, values := range ldapOperationalAttributes(p.DN) {
		attrs[k] = values
	}
	return attrs
}

func ldapEmptyGroupMemberFallbackDN(p PrincipalEntry, cn string) string {
	if dn := strings.TrimSpace(p.DN); dn != "" {
		return dn
	}
	name := firstNonEmpty(cn, p.DisplayName, p.UID, "empty-group")
	return "cn=" + escapeNormalizedDNValue(name)
}

func ldapProxyAddresses(mail string) []string {
	mail = strings.TrimSpace(mail)
	if mail == "" {
		return nil
	}
	return []string{"SMTP:" + mail}
}

func ldapObjectCategory(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "organization":
		return "organizationalUnit"
	case "group":
		return "group"
	case "resource":
		return "device"
	default:
		return "person"
	}
}

func nonEmptyLDAPValues(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			out = append(out, value)
		}
	}
	return out
}

func ldapOperationalAttributes(dn string) map[string][]string {
	dn = strings.TrimSpace(dn)
	if dn == "" {
		return nil
	}
	return map[string][]string{
		"entryDN":         {dn},
		"entryUUID":       {ldapEntryUUID(dn)},
		"hasSubordinates": {"FALSE"},
		"numSubordinates": {"0"},
		"createTimestamp": {"19700101000000Z"},
		"modifyTimestamp": {"19700101000000Z"},
		"creatorsName":    {"cn=gogomail"},
		"modifiersName":   {"cn=gogomail"},
	}
}

func ldapEntryUUID(dn string) string {
	b := ldapEntryUUIDBytes(dn)
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func ldapEntryUUIDBytes(dn string) []byte {
	sum := sha1.Sum([]byte(normalizeDNForCompare(dn)))
	b := sum[:16]
	b[6] = (b[6] & 0x0f) | 0x50
	b[8] = (b[8] & 0x3f) | 0x80
	out := make([]byte, 16)
	copy(out, b)
	return out
}

func ldapObjectSID(dn, kind string) string {
	a, c, d, rid, ok := ldapObjectSIDParts(dn, kind)
	if !ok {
		return ""
	}
	return fmt.Sprintf("S-1-5-21-%d-%d-%d-%d", a, c, d, rid)
}

func ldapObjectSIDBytes(dn, kind string) []byte {
	a, c, d, rid, ok := ldapObjectSIDParts(dn, kind)
	if !ok {
		return nil
	}
	out := []byte{1, 5, 0, 0, 0, 0, 0, 5}
	for _, v := range []uint32{21, a, c, d, rid} {
		out = append(out, byte(v), byte(v>>8), byte(v>>16), byte(v>>24))
	}
	return out
}

func ldapObjectSIDParts(dn, kind string) (uint32, uint32, uint32, uint32, bool) {
	if strings.TrimSpace(dn) == "" {
		return 0, 0, 0, 0, false
	}
	b := ldapEntryUUIDBytes(dn)
	a := uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
	c := uint32(b[4])<<24 | uint32(b[5])<<16 | uint32(b[6])<<8 | uint32(b[7])
	d := uint32(b[8])<<24 | uint32(b[9])<<16 | uint32(b[10])<<8 | uint32(b[11])
	rid := uint32(b[12])<<24 | uint32(b[13])<<16 | uint32(b[14])<<8 | uint32(b[15])
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "group":
		rid = 2000000000 + rid%999999999
	case "organization":
		rid = 3000000000 + rid%999999999
	case "resource":
		rid = 4000000000 + rid%294967295
	default:
		rid = 1000000000 + rid%999999999
	}
	return a, c, d, rid, true
}

func ldapUpdateSequenceNumber(dn string) string {
	b := ldapEntryUUIDBytes(dn)
	var n uint64
	for i := 0; i < 8 && i < len(b); i++ {
		n = (n << 8) | uint64(b[i])
	}
	if n == 0 {
		n = 1
	}
	return fmt.Sprintf("%d", n)
}

func ldapCanonicalName(dn string) string {
	parts := splitDNComponents(strings.TrimSpace(dn))
	if len(parts) == 0 {
		return ""
	}
	domain := make([]string, 0, 2)
	path := make([]string, 0, len(parts))
	for _, part := range parts {
		attr, value, ok := splitDNAttributeValue(part)
		if !ok || value == "" {
			return ""
		}
		if strings.EqualFold(attr, "dc") {
			domain = append(domain, value)
			continue
		}
		path = append([]string{value}, path...)
	}
	if len(domain) == 0 {
		return strings.Join(path, "/")
	}
	canonical := strings.Join(domain, ".")
	if len(path) > 0 {
		canonical += "/" + strings.Join(path, "/")
	}
	return canonical
}

func splitDNComponents(dn string) []string {
	if dn == "" {
		return nil
	}
	parts := make([]string, 0, 4)
	start := 0
	escaped := false
	for i := 0; i < len(dn); i++ {
		switch {
		case escaped:
			escaped = false
		case dn[i] == '\\':
			escaped = true
		case dn[i] == ',':
			parts = append(parts, strings.TrimSpace(dn[start:i]))
			start = i + 1
		}
	}
	parts = append(parts, strings.TrimSpace(dn[start:]))
	return parts
}

func splitDNAttributeValue(part string) (string, string, bool) {
	parts := strings.SplitN(strings.TrimSpace(part), "=", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	value, ok := unescapeDNValue(strings.TrimSpace(parts[1]))
	return strings.TrimSpace(parts[0]), value, ok
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func rootDSEAttributes(namingContexts []string, startTLSEnabled bool) map[string][]string {
	if len(namingContexts) == 0 {
		namingContexts = []string{"dc=local"}
	}
	defaultNamingContext := namingContexts[0]
	configurationNamingContext := "cn=Configuration," + defaultNamingContext
	serverName := "cn=ldap,cn=Servers,cn=Default-First-Site-Name,cn=Sites," + configurationNamingContext
	attrs := map[string][]string{
		"objectClass":                   {"top", "OpenLDAProotDSE"},
		"namingContexts":                namingContexts,
		"defaultNamingContext":          {defaultNamingContext},
		"rootDomainNamingContext":       {defaultNamingContext},
		"configurationNamingContext":    {configurationNamingContext},
		"schemaNamingContext":           {"cn=Schema," + configurationNamingContext},
		"subschemaSubentry":             {"cn=Subschema"},
		"supportedLDAPVersion":          {"3"},
		"supportedControl":              {controlManageDsaIT, controlPagedResults, controlServerSideSortRequest, controlVirtualListViewRequest, controlAssertion, controlMatchedValues, controlDomainScope, controlDontUseCopy, controlDontUseCopyOpenLDAP, controlSubentries, controlSyncRequest, controlProxiedAuthorization, controlDereferenceRequest, controlRelax, controlNoOp, controlPreRead, controlPostRead, controlPasswordPolicy, controlSessionTracking},
		"supportedExtension":            {whoAmIOID},
		"supportedFeatures":             {ldapFeatureAllOperationalAttributes},
		"supportedCapabilities":         {"1.2.840.113556.1.4.800", "1.2.840.113556.1.4.1670", "1.2.840.113556.1.4.1791"},
		"vendorName":                    {"gogomail"},
		"dnsHostName":                   {"ldap." + ldapDNSDomainFromNamingContext(defaultNamingContext)},
		"serverName":                    {serverName},
		"dsServiceName":                 {"cn=NTDS Settings," + serverName},
		"currentTime":                   {ldapGeneralizedTime(time.Now().UTC())},
		"highestCommittedUSN":           {ldapUpdateSequenceNumber(defaultNamingContext)},
		"domainControllerFunctionality": {"7"},
		"domainFunctionality":           {"7"},
		"forestFunctionality":           {"7"},
		"isGlobalCatalogReady":          {"TRUE"},
		"isSynchronized":                {"TRUE"},
	}
	if startTLSEnabled {
		attrs["supportedExtension"] = append(attrs["supportedExtension"], startTLSOID)
	}
	return attrs
}

func ldapGeneralizedTime(t time.Time) string {
	return t.UTC().Format("20060102150405") + ".0Z"
}

func ldapDNSDomainFromNamingContext(namingContext string) string {
	var labels []string
	for _, rdn := range splitDNComponents(namingContext) {
		parts := strings.SplitN(rdn, "=", 2)
		if len(parts) != 2 || !strings.EqualFold(strings.TrimSpace(parts[0]), "dc") {
			continue
		}
		label, ok := unescapeDNValue(strings.TrimSpace(parts[1]))
		if !ok {
			continue
		}
		label = strings.TrimSpace(label)
		if label != "" {
			labels = append(labels, label)
		}
	}
	if len(labels) == 0 {
		return "gogomail.local"
	}
	return strings.Join(labels, ".")
}

func subschemaAttributes() map[string][]string {
	return map[string][]string{
		"objectClass": {"top", "subschema"},
		"objectClasses": {
			"( 2.5.6.0 NAME 'top' ABSTRACT MUST objectClass )",
			"( 2.5.6.6 NAME 'person' SUP top STRUCTURAL MUST ( sn $ cn ) MAY ( userPassword $ telephoneNumber $ seeAlso $ description ) )",
			"( 2.5.6.7 NAME 'organizationalPerson' SUP person STRUCTURAL MAY ( title $ x121Address $ registeredAddress $ destinationIndicator $ preferredDeliveryMethod $ telexNumber $ teletexTerminalIdentifier $ telephoneNumber $ internationaliSDNNumber $ facsimileTelephoneNumber $ street $ postOfficeBox $ postalCode $ postalAddress $ physicalDeliveryOfficeName $ ou $ st $ l ) )",
			"( 2.16.840.1.113730.3.2.2 NAME 'inetOrgPerson' SUP organizationalPerson STRUCTURAL MAY ( mail $ uid $ givenName $ displayName $ name $ canonicalName $ distinguishedName $ instanceType $ objectCategory $ objectGUID $ objectSid $ mailNickname $ proxyAddresses $ sAMAccountName $ userPrincipalName $ whenCreated $ whenChanged $ uSNCreated $ uSNChanged $ accountExpires $ primaryGroupID $ userAccountControl ) )",
			"( 1.2.840.113556.1.5.9 NAME 'user' SUP organizationalPerson STRUCTURAL MAY ( mail $ uid $ givenName $ displayName $ name $ canonicalName $ distinguishedName $ instanceType $ objectCategory $ objectGUID $ objectSid $ mailNickname $ proxyAddresses $ sAMAccountName $ userPrincipalName $ whenCreated $ whenChanged $ uSNCreated $ uSNChanged $ accountExpires $ primaryGroupID $ userAccountControl ) )",
			"( 2.5.6.5 NAME 'organizationalUnit' SUP top STRUCTURAL MUST ou MAY ( userPassword $ searchGuide $ seeAlso $ businessCategory $ x121Address $ registeredAddress $ destinationIndicator $ preferredDeliveryMethod $ telexNumber $ teletexTerminalIdentifier $ telephoneNumber $ internationaliSDNNumber $ facsimileTelephoneNumber $ street $ postOfficeBox $ postalCode $ postalAddress $ physicalDeliveryOfficeName $ st $ l $ description $ canonicalName $ distinguishedName $ instanceType $ objectCategory $ objectGUID $ objectSid $ whenCreated $ whenChanged $ uSNCreated $ uSNChanged ) )",
			"( 2.5.6.9 NAME 'groupOfNames' SUP top STRUCTURAL MUST ( member $ cn ) MAY ( businessCategory $ seeAlso $ owner $ ou $ o $ description $ memberOf $ canonicalName $ distinguishedName $ instanceType $ objectCategory $ objectGUID $ objectSid $ whenCreated $ whenChanged $ uSNCreated $ uSNChanged ) )",
			"( 2.5.6.14 NAME 'device' SUP top STRUCTURAL MUST cn MAY ( serialNumber $ seeAlso $ owner $ ou $ o $ l $ description $ memberOf $ canonicalName $ distinguishedName $ instanceType $ objectCategory $ objectGUID $ objectSid $ whenCreated $ whenChanged $ uSNCreated $ uSNChanged ) )",
		},
		"attributeTypes": {
			"( 2.5.4.3 NAME 'cn' SUP name )",
			"( 2.5.4.4 NAME 'sn' SUP name )",
			"( 2.5.4.11 NAME 'ou' SUP name )",
			"( 2.5.4.13 NAME 'description' EQUALITY caseIgnoreMatch SUBSTR caseIgnoreSubstringsMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.15 )",
			"( 2.5.4.42 NAME 'givenName' SUP name )",
			"( 2.5.4.41 NAME 'name' EQUALITY caseIgnoreMatch SUBSTR caseIgnoreSubstringsMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.15 )",
			"( 0.9.2342.19200300.100.1.1 NAME 'uid' EQUALITY caseIgnoreMatch SUBSTR caseIgnoreSubstringsMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.15 )",
			"( 0.9.2342.19200300.100.1.3 NAME 'mail' EQUALITY caseIgnoreIA5Match SUBSTR caseIgnoreIA5SubstringsMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.26 )",
			"( 2.16.840.1.113730.3.1.241 NAME 'displayName' EQUALITY caseIgnoreMatch SUBSTR caseIgnoreSubstringsMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.15 )",
			"( 1.2.840.113556.1.4.221 NAME 'sAMAccountName' EQUALITY caseIgnoreMatch SUBSTR caseIgnoreSubstringsMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.15 )",
			"( 1.2.840.113556.1.4.656 NAME 'userPrincipalName' EQUALITY caseIgnoreMatch SUBSTR caseIgnoreSubstringsMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.15 )",
			"( 1.2.840.113556.1.4.7000.102.1 NAME 'mailNickname' EQUALITY caseIgnoreMatch SUBSTR caseIgnoreSubstringsMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.15 )",
			"( 1.2.840.113556.1.2.210 NAME 'proxyAddresses' EQUALITY caseIgnoreMatch SUBSTR caseIgnoreSubstringsMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.15 )",
			"( 1.2.840.113556.1.4.916 NAME 'canonicalName' EQUALITY caseIgnoreMatch SUBSTR caseIgnoreSubstringsMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.15 )",
			"( 2.5.4.49 NAME 'distinguishedName' EQUALITY distinguishedNameMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.12 )",
			"( 1.2.840.113556.1.2.1 NAME 'instanceType' EQUALITY integerMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.27 )",
			"( 1.2.840.113556.1.4.782 NAME 'objectCategory' EQUALITY caseIgnoreMatch SUBSTR caseIgnoreSubstringsMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.15 )",
			"( 1.2.840.113556.1.4.2 NAME 'objectGUID' EQUALITY octetStringMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.40 )",
			"( 1.2.840.113556.1.4.146 NAME 'objectSid' EQUALITY octetStringMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.40 )",
			"( 1.2.840.113556.1.2.2 NAME 'whenCreated' EQUALITY generalizedTimeMatch ORDERING generalizedTimeOrderingMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.24 )",
			"( 1.2.840.113556.1.2.3 NAME 'whenChanged' EQUALITY generalizedTimeMatch ORDERING generalizedTimeOrderingMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.24 )",
			"( 1.2.840.113556.1.2.19 NAME 'uSNCreated' EQUALITY integerMatch ORDERING integerOrderingMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.27 )",
			"( 1.2.840.113556.1.2.120 NAME 'uSNChanged' EQUALITY integerMatch ORDERING integerOrderingMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.27 )",
			"( 1.2.840.113556.1.4.159 NAME 'accountExpires' EQUALITY integerMatch ORDERING integerOrderingMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.27 )",
			"( 1.2.840.113556.1.4.98 NAME 'primaryGroupID' EQUALITY integerMatch ORDERING integerOrderingMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.27 )",
			"( 1.2.840.113556.1.4.8 NAME 'userAccountControl' EQUALITY integerMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.27 )",
		},
	}
}

func selectLDAPAttributes(attrs map[string][]string, requested []string, typesOnly bool) map[string][]string {
	selected := make(map[string][]string, len(attrs))
	if containsLDAPAttribute(requested, "1.1") {
		return selected
	}
	if len(requested) == 0 || containsLDAPAttribute(requested, "*") {
		copyLDAPAttributeSet(selected, attrs, typesOnly, false)
	}
	if containsLDAPAttribute(requested, "+") {
		copyLDAPAttributeSet(selected, attrs, typesOnly, true)
	}
	for _, name := range requested {
		name = ldapAttributeType(name)
		if name == "*" || name == "+" {
			continue
		}
		for attrName, values := range attrs {
			if strings.EqualFold(name, ldapAttributeType(attrName)) {
				if typesOnly {
					selected[attrName] = nil
				} else {
					selected[attrName] = values
				}
			}
		}
	}
	return selected
}

func copyLDAPAttributeSet(dst map[string][]string, attrs map[string][]string, typesOnly bool, operational bool) {
	for k, values := range attrs {
		if operational != isOperationalLDAPAttribute(k) {
			continue
		}
		if typesOnly {
			dst[k] = nil
		} else {
			dst[k] = values
		}
	}
}

func isOperationalLDAPAttribute(attr string) bool {
	switch strings.ToLower(ldapAttributeType(attr)) {
	case "subschemasubentry", "supportedldapversion", "supportedcontrol", "supportedextension", "supportedfeatures", "namingcontexts", "vendorname",
		"defaultnamingcontext", "rootdomainnamingcontext", "configurationnamingcontext", "schemanamingcontext", "supportedcapabilities",
		"dnshostname", "servername", "dsservicename", "currenttime", "highestcommittedusn",
		"domaincontrollerfunctionality", "domainfunctionality", "forestfunctionality", "isglobalcatalogready", "issynchronized",
		"entrydn", "entryuuid", "createtimestamp", "modifytimestamp", "creatorsname", "modifiersname",
		"distinguishedname", "objectguid", "objectsid", "instancetype", "whencreated", "whenchanged",
		"usncreated", "usnchanged", "hassubordinates", "numsubordinates":
		return true
	default:
		return false
	}
}

func containsLDAPAttribute(attrs []string, want string) bool {
	for _, attr := range attrs {
		if strings.EqualFold(ldapAttributeType(attr), want) {
			return true
		}
	}
	return false
}

func normalizeDNForCompare(dn string) string {
	components := splitDNComponents(strings.TrimSpace(dn))
	if len(components) == 0 {
		return ""
	}
	parts := make([]string, 0, len(components))
	for _, component := range components {
		attr, value, ok := splitDNAttributeValue(component)
		if !ok {
			parts = append(parts, strings.ToLower(strings.TrimSpace(component)))
			continue
		}
		parts = append(parts, strings.ToLower(strings.TrimSpace(attr))+"="+escapeNormalizedDNValue(strings.ToLower(value)))
	}
	return strings.Join(parts, ",")
}

func escapeNormalizedDNValue(value string) string {
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
