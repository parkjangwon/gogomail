import { useState, useEffect } from 'react';
import { listOrgTree, getUserProfile, OrgUnit } from '@/lib/api';

export interface UseOrgTreeResult {
  orgTree: OrgUnit[];
  selectedOrg: OrgUnit | null;
  setSelectedOrg: (unit: OrgUnit | null) => void;
  treeLoading: boolean;
  orgSearch: string;
  setOrgSearch: (s: string) => void;
  expandedIds: Set<string>;
  toggleExpanded: (id: string) => void;
  getChildrenOf: (parentId: string | undefined) => OrgUnit[];
  getRootOrgs: () => OrgUnit[];
  descendantOrgs: (orgId: string) => OrgUnit[];
  orgMemberCount: (unit: OrgUnit, includeChildren: boolean) => number;
  matchesSearch: (unit: OrgUnit) => boolean;
  q: string;
}

export function useOrgTree(): UseOrgTreeResult {
  const [orgTree, setOrgTree] = useState<OrgUnit[]>([]);
  const [selectedOrg, setSelectedOrg] = useState<OrgUnit | null>(null);
  const [treeLoading, setTreeLoading] = useState(false);
  const [orgSearch, setOrgSearch] = useState('');
  const [expandedIds, setExpandedIds] = useState<Set<string>>(new Set());

  // Load org tree on mount
  useEffect(() => {
    setTreeLoading(true);
    Promise.all([getUserProfile(), listOrgTree()])
      .then(([userProfile, units]) => {
        setOrgTree(units);

        // Find user's organization
        let userOrgId: string | null = null;
        if (userProfile) {
          for (const unit of units) {
            const member = unit.members.find((m) => m.id === userProfile.user_id);
            if (member) {
              userOrgId = unit.id;
              break;
            }
          }
        }

        // Build parent chain from user's org to root
        const toExpand = new Set<string>();
        if (userOrgId) {
          const userOrg = units.find((u) => u.id === userOrgId) ?? null;
          let current = userOrg;
          while (current && current.parent_id) {
            const parent = units.find((u) => u.id === current!.parent_id);
            if (parent) {
              toExpand.add(parent.id);
              current = parent;
            } else {
              break;
            }
          }
          if (userOrg && units.some((u) => u.parent_id === userOrg.id)) {
            toExpand.add(userOrg.id);
          }
          setSelectedOrg(userOrg);
        } else {
          units.filter((u: OrgUnit) => !u.parent_id).forEach((u: OrgUnit) => toExpand.add(u.id));
          setSelectedOrg(units.find((u: OrgUnit) => !u.parent_id) ?? null);
        }

        setExpandedIds(toExpand);
        setTreeLoading(false);
      })
      .catch(() => setTreeLoading(false));
  }, []);

  const toggleExpanded = (id: string) => {
    const next = new Set(expandedIds);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    setExpandedIds(next);
  };

  const getChildrenOf = (parentId: string | undefined): OrgUnit[] => {
    return orgTree.filter((u) => u.parent_id === parentId);
  };

  const getRootOrgs = (): OrgUnit[] => {
    return orgTree.filter((u) => !u.parent_id);
  };

  const descendantOrgs = (orgId: string): OrgUnit[] => {
    const children = getChildrenOf(orgId);
    return children.flatMap((child) => [child, ...descendantOrgs(child.id)]);
  };

  const orgMemberCount = (unit: OrgUnit, includeChildren: boolean): number => {
    if (!includeChildren) return unit.members.length;
    return unit.members.length + descendantOrgs(unit.id).reduce((sum, child) => sum + child.members.length, 0);
  };

  const q = orgSearch.trim().toLowerCase();

  const matchesSearch = (unit: OrgUnit): boolean => {
    if (!q) return true;
    return (
      unit.display_name.toLowerCase().includes(q) ||
      unit.members.some(
        (m) =>
          (m.display_name || '').toLowerCase().includes(q) ||
          m.email.toLowerCase().includes(q)
      )
    );
  };

  return {
    orgTree,
    selectedOrg,
    setSelectedOrg,
    treeLoading,
    orgSearch,
    setOrgSearch,
    expandedIds,
    toggleExpanded,
    getChildrenOf,
    getRootOrgs,
    descendantOrgs,
    orgMemberCount,
    matchesSearch,
    q,
  };
}
