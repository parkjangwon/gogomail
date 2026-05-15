'use client';

import { useQuery } from '@tanstack/react-query';
import { api } from '@/lib/api-client';
import type { components } from '@gogomail/api-types';

export type AdminConsoleCapabilities = components['schemas']['AdminConsoleCapabilities'];
export type AdminConsoleCapabilitiesEnvelope = components['schemas']['AdminConsoleCapabilitiesEnvelope'];

export function useConsoleCapabilities() {
  return useQuery({
    queryKey: ['admin', 'console-capabilities'],
    queryFn: async () => {
      const res = await api.get<AdminConsoleCapabilitiesEnvelope>('/console/capabilities');
      return res.admin_console_capabilities;
    },
    staleTime: 5 * 60 * 1000,
  });
}
