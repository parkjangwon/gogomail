import type { Dispatch, SetStateAction } from 'react';
import { getDriveUsage } from '@/lib/api';
import type { DriveUsage } from '@/lib/api';

export type DriveUsageSetter = Dispatch<SetStateAction<DriveUsage | null>>;

export function refreshDriveUsage(setUsage: DriveUsageSetter): void {
  void getDriveUsage().then(setUsage).catch(() => {});
}
