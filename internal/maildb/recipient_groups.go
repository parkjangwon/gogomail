package maildb

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/gogomail/gogomail/internal/outbound"
)

var vcardEmailLineRE = regexp.MustCompile(`(?im)^EMAIL[^:]*:([^\r\n]+)`)
var vcardFNLineRE = regexp.MustCompile(`(?im)^FN:([^\r\n]+)`)

func (r *Repository) ExpandOrgRecipients(ctx context.Context, userID string, orgID string, includeChildren bool) ([]outbound.Address, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	const query = `
WITH RECURSIVE actor AS (
  SELECT u.id AS user_id, d.id AS domain_id, c.id AS company_id
  FROM users u
  JOIN domains d ON d.id = u.domain_id
  JOIN companies c ON c.id = d.company_id
  WHERE u.id = $1::uuid
),
target_orgs AS (
  SELECT o.id
  FROM organizations o
  JOIN actor a ON a.domain_id = o.domain_id
  WHERE o.id = $2::uuid
    AND o.status = 'active'
  UNION ALL
  SELECT child.id
  FROM organizations child
  JOIN target_orgs parent ON child.parent_id = parent.id
  WHERE $3::boolean
    AND child.status = 'active'
),
primary_addresses AS (
  SELECT DISTINCT ON (ua.user_id)
         ua.user_id,
         ua.address
  FROM user_addresses ua
  JOIN actor a ON a.domain_id = ua.domain_id
  ORDER BY ua.user_id, ua.is_primary DESC, ua.created_at ASC, ua.id ASC
)
SELECT u.display_name, pa.address
FROM users u
JOIN actor a ON a.domain_id = u.domain_id
JOIN target_orgs o ON o.id = u.org_id
JOIN primary_addresses pa ON pa.user_id = u.id
WHERE u.status = 'active'
ORDER BY lower(u.display_name), lower(pa.address)`
	rows, err := r.db.QueryContext(ctx, query, strings.TrimSpace(userID), strings.TrimSpace(orgID), includeChildren)
	if err != nil {
		return nil, fmt.Errorf("expand organization recipients: %w", err)
	}
	defer rows.Close()
	var addresses []outbound.Address
	for rows.Next() {
		var address outbound.Address
		if err := rows.Scan(&address.Name, &address.Email); err != nil {
			return nil, fmt.Errorf("scan organization recipient: %w", err)
		}
		addresses = append(addresses, address)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate organization recipients: %w", err)
	}
	return addresses, nil
}

func (r *Repository) ExpandAddressBookRecipients(ctx context.Context, userID string, addressBookID string) ([]outbound.Address, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	const query = `
SELECT vcard
FROM carddav_contact_objects
WHERE user_id = $1::uuid
  AND addressbook_id = $2::uuid
  AND status = 'active'
ORDER BY lower(object_name), id`
	rows, err := r.db.QueryContext(ctx, query, strings.TrimSpace(userID), strings.TrimSpace(addressBookID))
	if err != nil {
		return nil, fmt.Errorf("expand address book recipients: %w", err)
	}
	defer rows.Close()
	var addresses []outbound.Address
	for rows.Next() {
		var vcard string
		if err := rows.Scan(&vcard); err != nil {
			return nil, fmt.Errorf("scan address book recipient: %w", err)
		}
		email := firstVCardMatch(vcardEmailLineRE, vcard)
		if strings.TrimSpace(email) == "" {
			continue
		}
		addresses = append(addresses, outbound.Address{
			Name:  firstVCardMatch(vcardFNLineRE, vcard),
			Email: email,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate address book recipients: %w", err)
	}
	return addresses, nil
}

func firstVCardMatch(re *regexp.Regexp, vcard string) string {
	match := re.FindStringSubmatch(vcard)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}
