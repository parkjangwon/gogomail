import { Box, Container, SpaceBetween } from '@cloudscape-design/components';
import type { BoxProps } from '@cloudscape-design/components';
import type { ReactNode } from 'react';

interface MetricCardProps {
  title: ReactNode;
  value: ReactNode;
  valueColor?: BoxProps.Color;
  description?: ReactNode;
  footer?: ReactNode;
  children?: ReactNode;
}

export function MetricCard({ title, value, valueColor, description, footer, children }: MetricCardProps) {
  return (
    <Container>
      <SpaceBetween size="xs">
        <Box color="text-body-secondary" fontSize="body-s">{title}</Box>
        <Box fontSize="display-l" fontWeight="bold" color={valueColor}>{value}</Box>
        {description && <Box color="text-body-secondary" fontSize="body-s">{description}</Box>}
        {children}
        {footer}
      </SpaceBetween>
    </Container>
  );
}
