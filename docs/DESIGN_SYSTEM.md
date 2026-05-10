# GoGoMail Admin Console - AWS-Inspired Design System

## Overview

The GoGoMail Admin Console now features an enterprise-grade dark theme design inspired by AWS Management Console. This document outlines the design system, components, and implementation guidelines.

## Color Palette

### Primary Colors
- **AWS Orange**: `#FF9900` - Primary action color
- **AWS Blue**: `#0972D3` - Information and secondary actions
- **AWS Green**: `#137633` - Success and healthy states
- **AWS Red**: `#D13212` - Error and critical states

### Semantic Colors
- **Success**: `#137633` (AWS Green) - Green badge: `#31A646`
- **Warning**: `#F57E25` (Orange) - Used for warnings and attention
- **Error**: `#ED1C24` (Bright Red) - Errors and critical issues
- **Info**: `#0972D3` (AWS Blue) - Informational content

### Dark Theme
- **Background Primary**: `#1A1A1A` - Main page background
- **Background Secondary**: `#232F3E` - Card and container background
- **Background Tertiary**: `#37475A` - Elevated surfaces
- **Border**: `#464646` - Default borders
- **Text Primary**: `#FFFFFF` - Main text
- **Text Secondary**: `#E8E8E8` - Secondary text
- **Text Tertiary**: `#CCCCCC` - Subtle text

## Component Patterns

### 1. Metric Cards

Display key metrics with status indicators:

```tsx
<div className="metric-card">
  <div style={{ fontSize: '12px', color: '#cccccc', textTransform: 'uppercase' }}>
    Total Users
  </div>
  <div style={{ fontSize: '28px', fontWeight: '700', color: '#0972d3' }}>
    1,250
  </div>
  <div style={{ fontSize: '12px', color: '#31a646', marginTop: '8px' }}>
    ↑ +12 this week
  </div>
</div>
```

### 2. Status Badges

Use semantic colors for different states:

```tsx
<span className="status-badge status-success">Active</span>
<span className="status-badge status-warning">Warning</span>
<span className="status-badge status-error">Error</span>
<span className="status-badge status-info">Info</span>
```

### 3. Container Elements

For cards and grouped content:

```tsx
<div className="card-aws">
  <h3>API Keys</h3>
  <p>Manage your API authentication keys</p>
</div>
```

### 4. Progress Bars

Use Cloudscape ProgressBar component for visual indicators:

```tsx
<ProgressBar value={65} label="Storage Usage: 65%" />
```

## CSS Classes

### Utility Classes

- `.container-aws` - Main container with border and shadow
- `.card-aws` - Card component with hover effects
- `.metric-card` - Metric display card
- `.status-badge` - Status indicator badge
- `.status-success`, `.status-warning`, `.status-error`, `.status-info` - Status variants
- `.gradient-orange` - AWS orange gradient text
- `.section-header` - Section header with divider
- `.pulse` - Loading/pulse animation

## Implementation Guidelines

### 1. Page Structure

```tsx
export default function PageName() {
  return (
    <ContentLayout
      header={
        <Header variant="h1" description="Page description">
          Page Title
        </Header>
      }
    >
      <SpaceBetween size="l">
        {/* Content sections */}
      </SpaceBetween>
    </ContentLayout>
  );
}
```

### 2. Metric Grid

```tsx
<Container header={<Header variant="h2">Key Metrics</Header>}>
  <ColumnLayout columns={4} variant="text-grid">
    <KeyValuePairs items={[{ label: 'Total Users', value: 1250 }]} />
    <KeyValuePairs items={[{ label: 'Active Users', value: 890 }]} />
    {/* More metrics */}
  </ColumnLayout>
</Container>
```

### 3. Data Tables

Cloudscape Tables automatically use dark theme. Add custom styling via CSS:

```tsx
<Table
  columnDefinitions={[...]}
  items={items}
  header={<Header variant="h2">Data Table</Header>}
/>
```

### 4. Forms & Input

Use Cloudscape FormField and Input components:

```tsx
<FormField label="Email Address">
  <Input placeholder="user@example.com" />
</FormField>
```

## Spacing System

- **xs**: 4px
- **sm**: 8px
- **md**: 16px (default)
- **lg**: 24px
- **xl**: 32px
- **xxl**: 48px

Use SpaceBetween component for consistent spacing:

```tsx
<SpaceBetween size="l">
  <Container>{...}</Container>
  <Container>{...}</Container>
</SpaceBetween>
```

## Typography

### Font Family
`'Amazon Ember', -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif`

### Font Sizes
- **xs**: 12px - Labels, badges
- **sm**: 14px - Secondary text
- **base**: 16px - Body text
- **lg**: 18px - Headings
- **xl**: 24px - Page titles
- **xxl**: 32px - Large headings

### Font Weights
- **Normal**: 400
- **Medium**: 500
- **Semibold**: 600
- **Bold**: 700

## Shadow System

- **sm**: `0 1px 2px rgba(0, 0, 0, 0.3)` - Subtle
- **md**: `0 4px 8px rgba(0, 0, 0, 0.3)` - Default
- **lg**: `0 8px 16px rgba(0, 0, 0, 0.4)` - Elevated
- **xl**: `0 16px 32px rgba(0, 0, 0, 0.5)` - High elevation

## Interactive States

### Hover Effects
- Border color change to AWS orange
- Shadow elevation increase
- Background color shift

### Focus States
- Outline: `0 0 0 2px rgba(255, 153, 0, 0.25)`
- All interactive elements must have clear focus states

## Responsive Design

All components use Cloudscape's responsive system:

- **Mobile**: Single column layouts
- **Tablet**: 2 column layouts (ColumnLayout columns={2})
- **Desktop**: 3-4 column layouts (ColumnLayout columns={4})

## Page Layout Pattern

```
┌─────────────────────────────────────┐
│ Header (Title + Actions)            │
├─────────────────────────────────────┤
│                                     │
│ Section 1: Key Metrics              │
│ - Metric cards in grid              │
│                                     │
├─────────────────────────────────────┤
│                                     │
│ Section 2: Main Content             │
│ - Data table or form                │
│                                     │
├─────────────────────────────────────┤
│                                     │
│ Section 3: Additional Info          │
│ - Charts, logs, details             │
│                                     │
└─────────────────────────────────────┘
```

## Common Patterns

### 1. Status Page
- Header with overall status
- Service status checklist
- Incident history (if any)
- Status metrics

### 2. Resource List
- Search/filter bar
- Table with actions
- Pagination controls
- Bulk actions (if needed)

### 3. Configuration Page
- Settings form with groups
- Save/Cancel buttons
- Validation feedback
- Related resources section

### 4. Dashboard
- Key metrics grid
- System health indicators
- Quick action cards
- Recent activity/logs

## Dark Mode Implementation

The entire application uses dark theme by default. No toggle needed - all CSS variables are set for dark mode.

To override in specific components, use CSS variables:

```css
/* Already defined in globals.css */
background-color: var(--bg-primary);
color: var(--text-primary);
border-color: var(--border-color);
```

## Accessibility

### WCAG 2.1 Level AA Compliance

- **Contrast Ratios**: All text meets 4.5:1 minimum
- **Focus States**: Visible on all interactive elements
- **Keyboard Navigation**: Full keyboard support via Cloudscape
- **ARIA Labels**: Used on all interactive elements
- **Color Not Alone**: Status never indicated by color alone

### Keyboard Shortcuts

- **Tab**: Navigate to next element
- **Shift+Tab**: Navigate to previous element
- **Enter/Space**: Activate buttons
- **Escape**: Close modals/dropdowns

## Best Practices

1. **Consistency**: Use the same components across all pages
2. **Spacing**: Always use SpaceBetween, never hardcode margins
3. **Colors**: Use semantic color names, not hex values in code
4. **Dark Theme**: Assume dark background, use light text
5. **Feedback**: Always provide loading, error, and success states
6. **Navigation**: Keep breadcrumbs and current location clear
7. **Performance**: Lazy load large datasets
8. **Testing**: Test in browser DevTools dark mode

## Migration Checklist

When updating existing pages:

- [ ] Add dark theme styling
- [ ] Update color values to use CSS variables
- [ ] Implement metric cards for key data
- [ ] Add status indicators where appropriate
- [ ] Update button styles to use CSS theme
- [ ] Test keyboard navigation
- [ ] Test with screen reader
- [ ] Verify contrast ratios
- [ ] Test on mobile viewport
- [ ] Add hover/focus states

## Files

- `src/styles/theme.ts` - Theme tokens
- `src/styles/globals.css` - Global styles and utilities
- `src/app/companies/[id]/dashboard/page.tsx` - Dashboard example

## Future Enhancements

- [ ] Data visualization charts (Recharts integration)
- [ ] Advanced filtering UI patterns
- [ ] Export/download functionality
- [ ] Real-time data updates (WebSocket)
- [ ] Advanced analytics dashboard
- [ ] Custom report builder
