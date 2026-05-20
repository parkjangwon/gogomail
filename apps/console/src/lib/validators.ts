export interface ValidationResult {
  valid: boolean;
  error?: string;
}

export function validateEmail(email: string): ValidationResult {
  const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
  if (!email) return { valid: false, error: "Email is required" };
  if (!emailRegex.test(email)) {
    return { valid: false, error: "Invalid email format" };
  }
  return { valid: true };
}

export function validatePassword(password: string): ValidationResult {
  if (!password) return { valid: false, error: "Password is required" };
  if (password.length < 8) {
    return { valid: false, error: "Password must be at least 8 characters" };
  }
  if (!/[A-Z]/.test(password)) {
    return { valid: false, error: "Password must contain uppercase letter" };
  }
  if (!/[a-z]/.test(password)) {
    return { valid: false, error: "Password must contain lowercase letter" };
  }
  if (!/[0-9]/.test(password)) {
    return { valid: false, error: "Password must contain number" };
  }
  return { valid: true };
}

export function validateName(name: string, minLength: number = 1): ValidationResult {
  if (!name) return { valid: false, error: "Name is required" };
  if (name.trim().length < minLength) {
    return {
      valid: false,
      error: `Name must be at least ${minLength} characters`,
    };
  }
  return { valid: true };
}

export function validateDomain(domain: string): ValidationResult {
  const domainRegex =
    /^(?:[a-z0-9](?:[a-z0-9-]*[a-z0-9])?\.)+[a-z]{2,}$/i;
  if (!domain) return { valid: false, error: "Domain is required" };
  if (!domainRegex.test(domain)) {
    return { valid: false, error: "Invalid domain format" };
  }
  return { valid: true };
}

export function validateUrl(url: string): ValidationResult {
  try {
    new URL(url);
    return { valid: true };
  } catch {
    return { valid: false, error: "Invalid URL format" };
  }
}

export function validateRequired(value: unknown): ValidationResult {
  if (!value) return { valid: false, error: "This field is required" };
  return { valid: true };
}

export function validateConnectionString(connStr: string): ValidationResult {
  if (!connStr) {
    return { valid: false, error: "Connection string is required" };
  }
  if (connStr.length < 10) {
    return {
      valid: false,
      error: "Connection string is too short",
    };
  }
  return { valid: true };
}

export function validateLDAPUrl(url: string): ValidationResult {
  if (!url) return { valid: false, error: "LDAP URL is required" };
  if (!url.startsWith("ldap://") && !url.startsWith("ldaps://")) {
    return {
      valid: false,
      error: "LDAP URL must start with ldap:// or ldaps://",
    };
  }
  try {
    new URL(url);
    return { valid: true };
  } catch {
    return { valid: false, error: "Invalid LDAP URL format" };
  }
}

export function validateRoleName(name: string): ValidationResult {
  if (!name) return { valid: false, error: "Role name is required" };
  if (name.length < 2) {
    return { valid: false, error: "Role name must be at least 2 characters" };
  }
  if (name.length > 50) {
    return { valid: false, error: "Role name must be at most 50 characters" };
  }
  return { valid: true };
}

export function validateDateRange(
  startDate: string,
  endDate: string
): ValidationResult {
  if (!startDate || !endDate) {
    return { valid: false, error: "Both dates are required" };
  }

  const start = new Date(startDate);
  const end = new Date(endDate);

  if (isNaN(start.getTime())) {
    return { valid: false, error: "Invalid start date" };
  }
  if (isNaN(end.getTime())) {
    return { valid: false, error: "Invalid end date" };
  }
  if (start >= end) {
    return { valid: false, error: "Start date must be before end date" };
  }

  return { valid: true };
}
