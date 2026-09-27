'use client';

import { createContext, useContext, useState, useEffect, useCallback } from 'react';

export interface Company {
  id: string;
  name: string;
  status: string;
  quota_used: number;
  quota_limit: number;
  quota_remaining: number;
  over_allocated: boolean;
  created_at: string;
}

// Admin session roles returned by /api/admin/auth/verify. Only these two roles
// may perform mutations; any other value (e.g. a future read-only role) must be
// treated as insufficient and destructive UI hidden/disabled.
export type AdminRole = 'system_admin' | 'company_admin' | string;

const MUTATING_ROLES: ReadonlySet<string> = new Set(['system_admin', 'company_admin']);

interface CompanyContextType {
  companies: Company[];
  currentCompany: Company | null;
  switchCompany: (id: string) => void;
  loading: boolean;
  refresh: () => void;
  /** Admin role from the verify endpoint, or null until resolved. */
  role: AdminRole | null;
  /** Authenticated admin's user id (used e.g. for self-delete guards). */
  currentUserId: string | null;
  /**
   * Whether the current session may perform destructive/mutating actions.
   * Frontend guard only — the backend remains the enforcer. Defaults to false
   * until the role has been resolved so we never briefly offer the gun.
   */
  canMutate: boolean;
  /** True only for the system_admin role (e.g. managing admin users). */
  isSystemAdmin: boolean;
}

const CompanyContext = createContext<CompanyContextType>({
  companies: [],
  currentCompany: null,
  switchCompany: () => {},
  loading: true,
  refresh: () => {},
  role: null,
  currentUserId: null,
  canMutate: false,
  isSystemAdmin: false,
});

export function CompanyProvider({
  children,
  initialCompanyId,
}: {
  children: React.ReactNode;
  initialCompanyId: string;
}) {
  const [companies, setCompanies] = useState<Company[]>([]);
  const [currentCompanyId, setCurrentCompanyId] = useState(initialCompanyId);
  const [loading, setLoading] = useState(true);
  const [role, setRole] = useState<AdminRole | null>(null);
  const [currentUserId, setCurrentUserId] = useState<string | null>(null);

  useEffect(() => {
    setCurrentCompanyId(initialCompanyId);
  }, [initialCompanyId]);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const res = await fetch('/api/admin/auth/verify', { credentials: 'include' });
        if (!res.ok) return;
        const data = (await res.json()) as { role?: string; user_id?: string };
        if (cancelled) return;
        setRole(data.role ?? null);
        setCurrentUserId(data.user_id ?? null);
      } catch {
        // Role stays null -> canMutate stays false (fail closed).
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  const fetchCompanies = useCallback(async () => {
    try {
      const res = await fetch('/api/admin/companies?limit=200', { credentials: 'include' });
      if (res.ok) {
        const data = await res.json();
        setCompanies(data.companies || []);
      }
    } catch {
      // ignore
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { fetchCompanies(); }, [fetchCompanies]);

  const currentCompany =
    companies.find(c => c.id === currentCompanyId) ??
    (currentCompanyId === 'default' ? companies[0] : null) ??
    null;

  const switchCompany = (id: string) => setCurrentCompanyId(id);

  const canMutate = role !== null && MUTATING_ROLES.has(role);
  const isSystemAdmin = role === 'system_admin';

  return (
    <CompanyContext.Provider
      value={{
        companies,
        currentCompany,
        switchCompany,
        loading,
        refresh: fetchCompanies,
        role,
        currentUserId,
        canMutate,
        isSystemAdmin,
      }}
    >
      {children}
    </CompanyContext.Provider>
  );
}

export function useCompany() {
  return useContext(CompanyContext);
}
