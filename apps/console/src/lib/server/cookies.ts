export const ADMIN_ACCESS_TOKEN_COOKIE = process.env.NODE_ENV === 'production'
  ? '__Host-admin_access_token'
  : 'admin_access_token';

export const LEGACY_ADMIN_ACCESS_TOKEN_COOKIE = 'admin_access_token';

