'use client';

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '@/lib/api-client';

export interface ReportSchedule {
  id: string;
  company_id: string;
  name: string;
  frequency: 'daily' | 'weekly' | 'monthly';
  template_type: string;
  recipients: string[];
  is_enabled: boolean;
  created_at: string;
  next_run: string;
}

export interface ReportTemplate {
  id: string;
  name: string;
  type: string;
  generated_at: string;
  file_size: number;
}

interface ReportsEnvelope {
  reports: ReportTemplate[];
}

export function useReportSchedules(companyId: string | undefined) {
  return useQuery({
    queryKey: ['reportSchedules', companyId],
    queryFn: async () => {
      if (!companyId) return [];
      return [];
    },
    enabled: !!companyId,
  });
}

export function useReportTemplates(companyId: string | undefined) {
  return useQuery({
    queryKey: ['reportTemplates', companyId],
    queryFn: async () => {
      if (!companyId) return [];
      const res = await api.get<ReportsEnvelope>('/reports');
      return res.reports;
    },
    enabled: !!companyId,
  });
}

export function useCreateReportSchedule() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (_variables: {
      companyId: string;
      data: Omit<ReportSchedule, 'id' | 'created_at'>;
    }) => {
      throw new Error('Report schedules are not supported by the current Admin API contract');
    },
    onSuccess: (_, { companyId }) => {
      queryClient.invalidateQueries({ queryKey: ['reportSchedules', companyId] });
    },
  });
}

export function useUpdateReportSchedule() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (_variables: {
      companyId: string;
      scheduleId: string;
      data: Partial<ReportSchedule>;
    }) => {
      throw new Error('Report schedule updates are not supported by the current Admin API contract');
    },
    onSuccess: (_, { companyId }) => {
      queryClient.invalidateQueries({ queryKey: ['reportSchedules', companyId] });
    },
  });
}

export function useDeleteReportSchedule() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (_variables: {
      companyId: string;
      scheduleId: string;
    }) => {
      throw new Error('Report schedule deletion is not supported by the current Admin API contract');
    },
    onSuccess: (_, { companyId }) => {
      queryClient.invalidateQueries({ queryKey: ['reportSchedules', companyId] });
    },
  });
}
