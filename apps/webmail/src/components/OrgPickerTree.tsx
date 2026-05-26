'use client';

import React from 'react';
import { OrgUnit } from '@/lib/api';

export interface RenderOrgTreeProps {
  units: OrgUnit[];
  getChildren: (parentId: string) => OrgUnit[];
  selectedOrg: OrgUnit | null;
  expandedIds: Set<string>;
  onToggleExpanded: (id: string) => void;
  onSelectOrg: (unit: OrgUnit) => void;
  rowHover: Record<string, (e: React.MouseEvent<HTMLElement>) => void>;
  depth?: number;
}

export function RenderOrgTree({
  units,
  getChildren,
  selectedOrg,
  expandedIds,
  onToggleExpanded,
  onSelectOrg,
  rowHover,
  depth = 0,
}: RenderOrgTreeProps) {
  return (
    <>
      {units.map((unit) => {
        const children = getChildren(unit.id);
        const isExpanded = expandedIds.has(unit.id);
        const isSelected = selectedOrg?.id === unit.id;

        const fontSize = depth === 0 ? 13 : depth === 1 ? 12 : 11;
        const fontWeight = depth === 0 ? 600 : depth === 1 ? 500 : 400;
        const textColor = depth === 0 ? 'var(--color-text-primary)' : 'var(--color-text-secondary)';
        const bgColor = !isSelected ? (
          depth === 0 ? 'transparent' :
          depth === 1 ? 'var(--color-bg-secondary)' :
          'var(--color-bg-tertiary)'
        ) : 'var(--color-accent-subtle)';

        return (
          <div key={unit.id}>
            <div
              onClick={() => {
                if (children.length > 0) onToggleExpanded(unit.id);
                onSelectOrg(unit);
              }}
              style={{
                display: 'flex', alignItems: 'center', gap: '4px',
                paddingTop: '8px', paddingBottom: '8px',
                paddingLeft: `${12 + depth * 24}px`, paddingRight: '12px',
                cursor: 'pointer',
                borderLeft: isSelected ? '3px solid var(--color-accent)' : '3px solid transparent',
                background: bgColor,
                fontWeight,
              }}
              {...(!isSelected ? rowHover : {})}
            >
              {children.length > 0 ? (
                <span
                  onClick={(e) => { e.stopPropagation(); onToggleExpanded(unit.id); }}
                  style={{
                    fontSize: '10px', color: 'var(--color-text-tertiary)', marginRight: '2px',
                    width: '12px', textAlign: 'center', cursor: 'pointer',
                  }}>
                  {isExpanded ? '▼' : '▶'}
                </span>
              ) : (
                <span style={{ fontSize: '10px', color: 'var(--color-text-tertiary)', marginRight: '2px', width: '12px', textAlign: 'center' }}>
                  {depth === 0 ? '▸' : '└'}
                </span>
              )}
              <span style={{ fontSize, color: isSelected ? 'var(--color-accent)' : textColor, flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                {unit.display_name}
              </span>
              <span style={{ fontSize: '11px', color: 'var(--color-text-tertiary)', flexShrink: 0 }}>{unit.members.length}</span>
            </div>

            {/* Children */}
            {isExpanded && children.length > 0 && (
              <RenderOrgTree
                units={children}
                getChildren={getChildren}
                selectedOrg={selectedOrg}
                expandedIds={expandedIds}
                onToggleExpanded={onToggleExpanded}
                onSelectOrg={onSelectOrg}
                rowHover={rowHover}
                depth={depth + 1}
              />
            )}
          </div>
        );
      })}
    </>
  );
}
