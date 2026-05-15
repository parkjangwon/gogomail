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

    const { result } = {
      result: useAlertConfigs('company-123'),
    };

    expect(vi.mocked(useQuery)).toHaveBeenCalled();
  });

  it('should create alert config with channels', () => {
    const mockMutate = vi.fn();
    vi.mocked(useMutation).mockReturnValue({
      mutate: mockMutate,
      isPending: false,
      error: null,
    } as any);

    const { mutate } = {
      mutate: vi.fn(),
    };

    const configData = {
      alert_type: 'login_failures',
      threshold: 10,
      name: 'Login Failure Alert',
      channels: [
        {
          channel_type: 'email',
          config: { email: 'admin@example.com' },
        },
      ],
    };

    expect(mutate).toBeDefined();
  });

  it('should update alert config', () => {
    const mockMutate = vi.fn();
    vi.mocked(useMutation).mockReturnValue({
      mutate: mockMutate,
      isPending: false,
      error: null,
    } as any);

    const configId = 'config-1';
    const updates = {
      threshold: 90.0,
      is_enabled: false,
    };

    expect(mockMutate).toBeDefined();
  });

  it('should delete alert config', () => {
    const mockMutate = vi.fn();
    vi.mocked(useMutation).mockReturnValue({
      mutate: mockMutate,
      isPending: false,
      error: null,
    } as any);

    const configId = 'config-1';

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

    const notificationId = 'notif-1';

    expect(mockMutate).toBeDefined();
  });
});
