package directory

import (
	"context"
	"fmt"
	"strings"

	"github.com/lib/pq"
)

const listOrgTreeBaseSQL = `
SELECT o.id::text, o.name, COALESCE(o.parent_id::text, ''), o.depth, o.order_index
FROM organizations o
JOIN domains d ON d.id = o.domain_id
JOIN companies c ON c.id = d.company_id`

func buildListOrgTreeQuery(companyID, domainID string) (string, []any) {
	args := []any{strings.TrimSpace(companyID)}
	conditions := []string{"c.id = $1::uuid", "o.status = 'active'"}
	if domainID = strings.TrimSpace(domainID); domainID != "" {
		args = append(args, domainID)
		conditions = append(conditions, fmt.Sprintf("d.id = $%d::uuid", len(args)))
	}
	query := listOrgTreeBaseSQL + "\nWHERE " + strings.Join(conditions, "\n  AND ") + `
ORDER BY o.depth, o.order_index, lower(o.name)`
	return query, args
}

func (r *Repository) ListOrgTree(ctx context.Context, companyID, domainID string) ([]OrgTreeItem, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	query, args := buildListOrgTreeQuery(companyID, domainID)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list org tree: %w", err)
	}
	defer rows.Close()
	var result []OrgTreeItem
	for rows.Next() {
		var item OrgTreeItem
		if err := rows.Scan(&item.ID, &item.DisplayName, &item.ParentID, &item.Depth, &item.OrderIndex); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func buildListOrgMembersByOrgIDsQuery(domainID string) string {
	domainPredicate := ""
	orgIDsArg := 2
	limitArg := 3
	if strings.TrimSpace(domainID) != "" {
		domainPredicate = "\n    AND d.id = $2::uuid"
		orgIDsArg = 3
		limitArg = 4
	}
	return fmt.Sprintf(`
WITH requested AS (
  SELECT org_id::uuid, org_order
  FROM unnest($%d::text[]) WITH ORDINALITY AS req(org_id, org_order)
),
ranked AS (
  SELECT
    req.org_id::text AS organization_id,
    u.id::text AS id,
    c.id::text AS company_id,
    d.id::text AS domain_id,
    COALESCE(u.org_id::text, '') AS user_org_id,
    u.display_name,
    COALESCE(primary_addr.address, '') AS primary_email,
    COALESCE(u.settings->>'avatar_url', '') AS avatar_url,
    u.status,
    row_number() OVER (PARTITION BY req.org_id ORDER BY lower(u.display_name), u.id) AS member_rank,
    req.org_order
  FROM requested req
  JOIN organizations o ON o.id = req.org_id
  JOIN domains d ON d.id = o.domain_id
  JOIN companies c ON c.id = d.company_id
  JOIN users u ON u.org_id = o.id AND u.domain_id = d.id
  LEFT JOIN user_addresses primary_addr ON primary_addr.user_id = u.id AND primary_addr.is_primary = true
  WHERE c.id = $1::uuid%s
    AND o.status = 'active'
    AND u.status = 'active'
    AND d.status = 'active'
    AND c.status = 'active'
)
SELECT organization_id, id, company_id, domain_id, user_org_id, display_name, primary_email, avatar_url, status
FROM ranked
WHERE member_rank <= $%d
ORDER BY org_order, member_rank`, orgIDsArg, domainPredicate, limitArg)
}

func (r *Repository) ListOrgMembersByOrgIDs(ctx context.Context, companyID, domainID string, orgIDs []string, limitPerOrg int) (map[string][]Principal, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	companyID, err := NormalizePrincipalID(companyID)
	if err != nil {
		return nil, fmt.Errorf("company id: %w", err)
	}
	if strings.TrimSpace(domainID) != "" {
		domainID, err = NormalizePrincipalID(domainID)
		if err != nil {
			return nil, fmt.Errorf("domain id: %w", err)
		}
	}
	orgIDs, err = normalizePrincipalIDList(orgIDs)
	if err != nil {
		return nil, fmt.Errorf("organization ids: %w", err)
	}
	out := make(map[string][]Principal, len(orgIDs))
	if len(orgIDs) == 0 {
		return out, nil
	}
	if limitPerOrg <= 0 {
		limitPerOrg = MaxPrincipalSearchLimit
	}
	if limitPerOrg > MaxPrincipalSearchLimit {
		limitPerOrg = MaxPrincipalSearchLimit
	}
	args := []any{companyID}
	if domainID != "" {
		args = append(args, domainID)
	}
	args = append(args, pq.Array(orgIDs), limitPerOrg)
	rows, err := r.db.QueryContext(ctx, buildListOrgMembersByOrgIDsQuery(domainID), args...)
	if err != nil {
		return nil, fmt.Errorf("list org members by org ids: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var orgID string
		principal := Principal{Kind: PrincipalKindUser}
		if err := rows.Scan(
			&orgID,
			&principal.ID,
			&principal.CompanyID,
			&principal.DomainID,
			&principal.OrganizationID,
			&principal.DisplayName,
			&principal.PrimaryEmail,
			&principal.AvatarURL,
			&principal.Status,
		); err != nil {
			return nil, fmt.Errorf("scan org member: %w", err)
		}
		out[orgID] = append(out[orgID], principal)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list org members rows: %w", err)
	}
	return out, nil
}

func (r *Repository) SearchPrincipals(ctx context.Context, req SearchPrincipalsRequest) ([]Principal, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	req, err := NormalizeSearchPrincipalsRequest(req)
	if err != nil {
		return nil, err
	}
	query, args := buildSearchPrincipalsQuery(req)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("search directory principals: %w", err)
	}
	defer rows.Close()
	principals := make([]Principal, 0, req.Limit)
	for rows.Next() {
		var principal Principal
		if err := rows.Scan(
			&principal.ID,
			&principal.Kind,
			&principal.CompanyID,
			&principal.DomainID,
			&principal.OrganizationID,
			&principal.DisplayName,
			&principal.PrimaryEmail,
			&principal.AvatarURL,
			&principal.Status,
			&principal.ResourceType,
		); err != nil {
			return nil, fmt.Errorf("scan directory principal search result: %w", err)
		}
		principals = append(principals, principal)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("search directory principal rows: %w", err)
	}
	return principals, nil
}

func buildSearchPrincipalsQuery(req SearchPrincipalsRequest) (string, []any) {
	args := []any{req.CompanyID}
	var domainArg, orgArg, patternArg int
	if req.DomainID != "" {
		args = append(args, req.DomainID)
		domainArg = len(args)
	}
	if req.OrganizationID != "" {
		args = append(args, req.OrganizationID)
		orgArg = len(args)
	}
	if pattern := principalSearchPattern(req.Query); pattern != "" {
		args = append(args, pattern)
		patternArg = len(args)
	}

	branches := make([]string, 0, len(req.Kinds))
	if searchPrincipalKindAllowed(req.Kinds, PrincipalKindUser) {
		conditions := []string{"c.id = $1::uuid"}
		if domainArg != 0 {
			conditions = append(conditions, fmt.Sprintf("d.id = $%d::uuid", domainArg))
		}
		if orgArg != 0 {
			conditions = append(conditions, fmt.Sprintf("u.org_id = $%d::uuid", orgArg))
		}
		if req.ActiveOnly {
			conditions = append(conditions, "(u.status = 'active' AND d.status = 'active' AND c.status = 'active')")
		}
		if patternArg != 0 {
			conditions = append(conditions, fmt.Sprintf("(lower(u.display_name) LIKE $%d ESCAPE '\\' OR lower(u.username) LIKE $%d ESCAPE '\\' OR lower(COALESCE(primary_addr.address, '')) LIKE $%d ESCAPE '\\')", patternArg, patternArg, patternArg))
		}
		branches = append(branches, fmt.Sprintf(`
  SELECT u.id::text AS id,
         'user' AS kind,
         c.id::text AS company_id,
         d.id::text AS domain_id,
         COALESCE(u.org_id::text, '') AS organization_id,
         u.display_name AS display_name,
         COALESCE(primary_addr.address, '') AS primary_email,
         COALESCE(u.settings->>'avatar_url', '') AS avatar_url,
         u.status AS status,
         '' AS resource_type,
         1 AS kind_rank
  FROM users u
  JOIN domains d ON d.id = u.domain_id
  JOIN companies c ON c.id = d.company_id
  LEFT JOIN LATERAL (
    SELECT address
    FROM user_addresses
    WHERE user_id = u.id
    ORDER BY is_primary DESC, created_at ASC, id ASC
    LIMIT 1
  ) primary_addr ON TRUE
  WHERE %s`, strings.Join(conditions, "\n    AND ")))
	}
	if searchPrincipalKindAllowed(req.Kinds, PrincipalKindOrganization) {
		conditions := []string{"c.id = $1::uuid"}
		if domainArg != 0 {
			conditions = append(conditions, fmt.Sprintf("d.id = $%d::uuid", domainArg))
		}
		if orgArg != 0 {
			conditions = append(conditions, fmt.Sprintf("o.id = $%d::uuid", orgArg))
		}
		if req.ActiveOnly {
			conditions = append(conditions, "(o.status = 'active' AND d.status = 'active' AND c.status = 'active')")
		}
		if patternArg != 0 {
			conditions = append(conditions, fmt.Sprintf("(lower(o.name) LIKE $%d ESCAPE '\\' OR lower(o.code) LIKE $%d ESCAPE '\\')", patternArg, patternArg))
		}
		branches = append(branches, fmt.Sprintf(`
  SELECT o.id::text,
         'organization',
         c.id::text,
         d.id::text,
         o.id::text,
         o.name,
         '',
         '',
         o.status,
         '',
         2
  FROM organizations o
  JOIN domains d ON d.id = o.domain_id
  JOIN companies c ON c.id = d.company_id
  WHERE %s`, strings.Join(conditions, "\n    AND ")))
	}
	if searchPrincipalKindAllowed(req.Kinds, PrincipalKindGroup) {
		conditions := []string{"g.company_id = $1::uuid"}
		if domainArg != 0 {
			conditions = append(conditions, fmt.Sprintf("g.domain_id = $%d::uuid", domainArg))
		}
		if orgArg != 0 {
			conditions = append(conditions, fmt.Sprintf("g.org_id = $%d::uuid", orgArg))
		}
		if req.ActiveOnly {
			conditions = append(conditions, "(g.status = 'active' AND d.status = 'active' AND c.status = 'active')")
		}
		if patternArg != 0 {
			conditions = append(conditions, fmt.Sprintf("(lower(g.name) LIKE $%d ESCAPE '\\' OR lower(g.slug) LIKE $%d ESCAPE '\\')", patternArg, patternArg))
		}
		branches = append(branches, fmt.Sprintf(`
  SELECT g.id::text,
         'group',
         g.company_id::text,
         g.domain_id::text,
         COALESCE(g.org_id::text, ''),
         g.name,
         '',
         '',
         g.status,
         '',
         3
  FROM directory_groups g
  JOIN domains d ON d.id = g.domain_id
  JOIN companies c ON c.id = g.company_id AND c.id = d.company_id
  WHERE %s`, strings.Join(conditions, "\n    AND ")))
	}
	if searchPrincipalKindAllowed(req.Kinds, PrincipalKindResource) {
		conditions := []string{"rsrc.company_id = $1::uuid"}
		if domainArg != 0 {
			conditions = append(conditions, fmt.Sprintf("rsrc.domain_id = $%d::uuid", domainArg))
		}
		if orgArg != 0 {
			conditions = append(conditions, fmt.Sprintf("rsrc.org_id = $%d::uuid", orgArg))
		}
		if req.ActiveOnly {
			conditions = append(conditions, "(rsrc.status = 'active' AND d.status = 'active' AND c.status = 'active')")
		}
		if patternArg != 0 {
			conditions = append(conditions, fmt.Sprintf("(lower(rsrc.name) LIKE $%d ESCAPE '\\' OR lower(rsrc.slug) LIKE $%d ESCAPE '\\' OR lower(rsrc.resource_type) LIKE $%d ESCAPE '\\')", patternArg, patternArg, patternArg))
		}
		branches = append(branches, fmt.Sprintf(`
  SELECT rsrc.id::text,
         'resource',
         rsrc.company_id::text,
         rsrc.domain_id::text,
         COALESCE(rsrc.org_id::text, ''),
         rsrc.name,
         '',
         '',
         rsrc.status,
         rsrc.resource_type,
         4
  FROM directory_resources rsrc
  JOIN domains d ON d.id = rsrc.domain_id
  JOIN companies c ON c.id = rsrc.company_id AND c.id = d.company_id
  WHERE %s`, strings.Join(conditions, "\n    AND ")))
	}

	args = append(args, req.Limit, req.Offset)
	limitArg := len(args) - 1
	offsetArg := len(args)
	query := `
WITH principals AS (` + strings.Join(branches, "\n  UNION ALL") + `
)
SELECT id, kind, company_id, domain_id, organization_id, display_name, primary_email, avatar_url, status, resource_type
FROM principals
ORDER BY kind_rank, lower(display_name), id
LIMIT $%d OFFSET $%d`
	return fmt.Sprintf(query, limitArg, offsetArg), args
}

func searchPrincipalKindAllowed(kinds []string, kind string) bool {
	for _, candidate := range kinds {
		if candidate == kind {
			return true
		}
	}
	return false
}

func principalSearchPattern(query string) string {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(query) + 2)
	b.WriteByte('%')
	for _, r := range query {
		switch r {
		case '%', '_', '\\':
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	b.WriteByte('%')
	return b.String()
}
