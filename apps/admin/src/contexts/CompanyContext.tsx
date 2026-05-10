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

interface CompanyContextType {
  companies: Company[];
  currentCompany: Company | null;
  switchCompany: (id: string) => void;
  loading: boolean;
  refresh: () => void;
}

const CompanyContext = createContext<CompanyContextType>({
  companies: [],
  currentCompany: null,
  switchCompany: () => {},
  loading: true,
  refresh: () => {},
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

  return (
    <CompanyContext.Provider value={{ companies, currentCompany, switchCompany, loading, refresh: fetchCompanies }}>
      {children}
    </CompanyContext.Provider>
  );
}

export function useCompany() {
  return useContext(CompanyContext);
}
