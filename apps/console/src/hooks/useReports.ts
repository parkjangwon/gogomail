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
  description: string;
  sections: string[];
  created_at: string;
}

export function useReportSchedules(companyId: string | undefined) {
  return useQuery({
    queryKey: ['reportSchedules', companyId],
    queryFn: async () => {
      if (!companyId) return [];
      const res = await api.get(`/companies/${companyId}/reports/schedules`) as any;
      return (res.data?.schedules || []) as ReportSchedule[];
    },
    enabled: !!companyId,
  });
}

export function useReportTemplates(companyId: string | undefined) {
  return useQuery({
    queryKey: ['reportTemplates', companyId],
    queryFn: async () => {
      if (!companyId) return [];
      const res = await api.get(`/companies/${companyId}/reports/templates`) as any;
      return (res.data?.templates || []) as ReportTemplate[];
    },
    enabled: !!companyId,
  });
}

export function useCreateReportSchedule() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({
      companyId,
      data,
    }: {
      companyId: string;
      data: Omit<ReportSchedule, 'id' | 'created_at'>;
    }) => {
      const res = await api.post(
        `/companies/${companyId}/reports/schedules`,
        data
      ) as any;
      return res.data as ReportSchedule;
    },
    onSuccess: (_, { companyId }) => {
      queryClient.invalidateQueries({ queryKey: ['reportSchedules', companyId] });
    },
  });
}

export function useUpdateReportSchedule() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({
      companyId,
      scheduleId,
      data,
    }: {
      companyId: string;
      scheduleId: string;
      data: Partial<ReportSchedule>;
    }) => {
      const res = await api.put(
        `/companies/${companyId}/reports/schedules/${scheduleId}`,
        data
      ) as any;
      return res.data as ReportSchedule;
    },
    onSuccess: (_, { companyId }) => {
      queryClient.invalidateQueries({ queryKey: ['reportSchedules', companyId] });
    },
  });
}

export function useDeleteReportSchedule() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({
      companyId,
      scheduleId,
    }: {
      companyId: string;
      scheduleId: string;
    }) => {
      const res = await api.delete(
        `/companies/${companyId}/reports/schedules/${scheduleId}`
      ) as any;
      return res.data;
    },
    onSuccess: (_, { companyId }) => {
      queryClient.invalidateQueries({ queryKey: ['reportSchedules', companyId] });
    },
  });
}
