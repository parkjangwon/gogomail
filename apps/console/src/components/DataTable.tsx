'use client';

import { useEffect, useMemo, useState } from 'react';
import {
  Pagination,
  Table,
  TextFilter,
  type TableProps,
} from '@cloudscape-design/components';
import { useI18n } from '@/app/i18n-provider';

type DataTableProps<T> = Omit<TableProps<T>, 'items'> & {
  items: readonly T[];
  pageSize?: number;
  searchPlaceholder?: string;
};

const DEFAULT_PAGE_SIZE = 25;

function rowText(item: unknown): string {
  if (item == null) return '';
  if (typeof item === 'string' || typeof item === 'number' || typeof item === 'boolean') {
    return String(item);
  }
  try {
    return JSON.stringify(item);
  } catch {
    return String(item);
  }
}

export function DataTable<T>({
  items,
  pageSize = DEFAULT_PAGE_SIZE,
  searchPlaceholder,
  filter,
  pagination,
  ...props
}: DataTableProps<T>) {
  const { t } = useI18n();
  const [filteringText, setFilteringText] = useState('');
  const [currentPageIndex, setCurrentPageIndex] = useState(1);

  const hasCustomFilter = filter !== undefined;
  const hasCustomPagination = pagination !== undefined;

  const filteredItems = useMemo(() => {
    if (hasCustomFilter || filteringText.trim() === '') return [...items];
    const needle = filteringText.trim().toLowerCase();
    return items.filter(item => rowText(item).toLowerCase().includes(needle));
  }, [filteringText, hasCustomFilter, items]);

  const pagesCount = Math.max(1, Math.ceil(filteredItems.length / pageSize));
  const safePageIndex = Math.min(currentPageIndex, pagesCount);

  useEffect(() => {
    if (currentPageIndex !== safePageIndex) {
      setCurrentPageIndex(safePageIndex);
    }
  }, [currentPageIndex, safePageIndex]);

  useEffect(() => {
    setCurrentPageIndex(1);
  }, [filteringText, items.length]);

  const displayedItems = hasCustomPagination
    ? filteredItems
    : filteredItems.slice((safePageIndex - 1) * pageSize, safePageIndex * pageSize);

  return (
    <div className="admin-data-table">
      <Table
        {...props}
        items={displayedItems as T[]}
        filter={
          filter ?? (
            <TextFilter
              filteringText={filteringText}
              filteringPlaceholder={searchPlaceholder ?? t('common.search')}
              onChange={({ detail }) => setFilteringText(detail.filteringText)}
            />
          )
        }
        pagination={
          pagination ?? (
            <Pagination
              currentPageIndex={safePageIndex}
              pagesCount={pagesCount}
              onChange={({ detail }) => setCurrentPageIndex(detail.currentPageIndex)}
            />
          )
        }
      />
    </div>
  );
}
