import type { Dispatch, SetStateAction } from 'react';
import { getDriveUsage } from '@/lib/api';
import type { DriveUsage } from '@/lib/api';
import { ignoreNonCritical } from '@/lib/promise';

export type DriveUsageSetter = Dispatch<SetStateAction<DriveUsage | null>>;

export function refreshDriveUsage(setUsage: DriveUsageSetter): void {
  ignoreNonCritical(getDriveUsage().then(setUsage), 'drive.usage.refresh');
}
