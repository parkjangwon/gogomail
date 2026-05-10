// AWS Console Inspired Theme
export const theme = {
  // Color Palette
  colors: {
    // Primary
    primary: '#FF9900', // AWS Orange
    primaryHover: '#EC7211',
    primaryActive: '#FF9900',

    // Grays (Dark theme)
    background: '#1A1A1A',
    backgroundSecondary: '#232F3E',
    backgroundTertiary: '#37475A',

    foreground: '#FFFFFF',
    foregroundSecondary: '#E8E8E8',
    foregroundTertiary: '#CCCCCC',

    // Borders
    border: '#464646',
    borderLight: '#565656',
    borderDark: '#242F3E',

    // Status Colors
    success: '#137633', // AWS Green
    successLight: '#31A646',

    warning: '#D13212', // AWS Orange-Red
    warningLight: '#F57E25',

    error: '#D13212',
    errorLight: '#ED1C24',

    info: '#0972D3', // AWS Blue
    infoLight: '#1F88D9',

    critical: '#9C27B0', // Purple

    // Accents
    accent: '#00A1DE',
    accentDark: '#006CB3',

    // Semantic
    positive: '#137633',
    neutral: '#CCCCCC',
    negative: '#D13212',
  },

  // Spacing
  spacing: {
    xs: '4px',
    sm: '8px',
    md: '16px',
    lg: '24px',
    xl: '32px',
    xxl: '48px',
  },

  // Typography
  typography: {
    // Font families
    fontFamily: "'Amazon Ember', -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif",

    // Font sizes
    fontSize: {
      xs: '12px',
      sm: '14px',
      base: '16px',
      lg: '18px',
      xl: '24px',
      xxl: '32px',
    },

    // Font weights
    fontWeight: {
      normal: 400,
      medium: 500,
      semibold: 600,
      bold: 700,
    },

    // Line heights
    lineHeight: {
      tight: 1.2,
      normal: 1.5,
      relaxed: 1.75,
    },
  },

  // Shadows
  shadows: {
    sm: '0 1px 2px rgba(0, 0, 0, 0.3)',
    md: '0 4px 8px rgba(0, 0, 0, 0.3)',
    lg: '0 8px 16px rgba(0, 0, 0, 0.4)',
    xl: '0 16px 32px rgba(0, 0, 0, 0.5)',
  },

  // Border radius
  borderRadius: {
    sm: '2px',
    md: '4px',
    lg: '8px',
    xl: '12px',
  },

  // Transitions
  transitions: {
    fast: '150ms ease-in-out',
    normal: '250ms ease-in-out',
    slow: '350ms ease-in-out',
  },

  // Z-index
  zIndex: {
    dropdown: 1000,
    sticky: 1020,
    fixed: 1030,
    modalBackdrop: 1040,
    modal: 1050,
    popover: 1060,
    tooltip: 1070,
  },
} as const;

export type Theme = typeof theme;
