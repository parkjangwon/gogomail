import { afterEach, describe, expect, it, vi } from 'vitest';
import { requiredBackendUrl } from '../server/backend';

const originalBackendURL = process.env.GOGOMAIL_BACKEND_URL;
const originalAdminBackendURL = process.env.ADMIN_BACKEND_URL;

afterEach(() => {
  process.env.GOGOMAIL_BACKEND_URL = originalBackendURL;
  process.env.ADMIN_BACKEND_URL = originalAdminBackendURL;
  vi.unstubAllEnvs();
});

describe('server backend URL config', () => {
  it('requires explicit backend URL configuration in production', () => {
    delete process.env.GOGOMAIL_BACKEND_URL;
    delete process.env.ADMIN_BACKEND_URL;
    vi.stubEnv('NODE_ENV', 'production');

    expect(() => requiredBackendUrl()).toThrow(/GOGOMAIL_BACKEND_URL is required/);
  });

  it('normalizes configured backend URL and keeps env precedence', () => {
    process.env.GOGOMAIL_BACKEND_URL = 'https://api.example.test/';
    process.env.ADMIN_BACKEND_URL = 'https://legacy.example.test/';

    expect(requiredBackendUrl()).toBe('https://api.example.test');
  });
});
