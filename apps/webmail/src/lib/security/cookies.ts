export const WEBMAIL_TOKEN_COOKIE = process.env.NODE_ENV === 'production'
  ? '__Host-webmail_token'
  : 'webmail_token';

export const LEGACY_WEBMAIL_TOKEN_COOKIE = 'webmail_token';

