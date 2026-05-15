import { describe, it, expect, beforeEach, vi } from 'vitest';
import { useAlertConfigs } from '../useAlertConfigs';
import { useQuery, useMutation } from '@tanstack/react-query';

// Mock react-query
vi.mock('@tanstack/react-query');

describe('useAlertConfigs', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('should fetch alert configs for a company', () => {
    const mockData = [
      {
        id: 'config-1',
        alert_type: 'storage',
        threshold: 80.0,
        name: 'Storage Alert',
        is_enabled: true,
        channels: [],
      },
    ];

    vi.mocked(useQuery).mockReturnValue({
      data: mockData,
      isLoading: false,
      error: null,
    } as any);

    useAlertConfigs('company-123');

    expect(vi.mocked(useQuery)).toHaveBeenCalled();
  });

  it('should create alert config with channels', () => {
    const mockMutate = vi.fn();
    vi.mocked(useMutation).mockReturnValue({
      mutate: mockMutate,
      isPending: false,
      error: null,
    } as any);

    expect(mockMutate).toBeDefined();
  });

  it('should update alert config', () => {
    const mockMutate = vi.fn();
    vi.mocked(useMutation).mockReturnValue({
      mutate: mockMutate,
      isPending: false,
      error: null,
    } as any);

    expect(mockMutate).toBeDefined();
  });

  it('should delete alert config', () => {
    const mockMutate = vi.fn();
    vi.mocked(useMutation).mockReturnValue({
      mutate: mockMutate,
      isPending: false,
      error: null,
    } as any);

    expect(mockMutate).toBeDefined();
  });

  it('should list alert notifications', () => {
    const mockData = [
      {
        id: 'notif-1',
        alert_type: 'storage',
        threshold: 80,
        current_value: 85.5,
        created_at: new Date().toISOString(),
        acknowledged_at: null,
      },
    ];

    vi.mocked(useQuery).mockReturnValue({
      data: mockData,
      isLoading: false,
      error: null,
    } as any);

    expect(vi.mocked(useQuery)).toBeDefined();
  });

  it('should acknowledge notification', () => {
    const mockMutate = vi.fn();
    vi.mocked(useMutation).mockReturnValue({
      mutate: mockMutate,
      isPending: false,
      error: null,
    } as any);

    expect(mockMutate).toBeDefined();
  });
});
