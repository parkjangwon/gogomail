import { describe, it, expect } from "vitest";
import {
  validateEmail,
  validatePassword,
  validateName,
  validateDomain,
  validateUrl,
  validateConnectionString,
  validateLDAPUrl,
  validateRoleName,
  validateDateRange,
} from "../validators";

describe("validators", () => {
  describe("validateEmail", () => {
    it("should validate correct email", () => {
      const result = validateEmail("user@example.com");
      expect(result.valid).toBe(true);
    });

    it("should reject empty email", () => {
      const result = validateEmail("");
      expect(result.valid).toBe(false);
      expect(result.error).toBe("Email is required");
    });

    it("should reject invalid email format", () => {
      const result = validateEmail("invalid-email");
      expect(result.valid).toBe(false);
    });
  });

  describe("validatePassword", () => {
    it("should validate strong password", () => {
      const result = validatePassword("SecurePass123");
      expect(result.valid).toBe(true);
    });

    it("should reject password without uppercase", () => {
      const result = validatePassword("securepass123");
      expect(result.valid).toBe(false);
    });

    it("should reject password without lowercase", () => {
      const result = validatePassword("SECUREPASS123");
      expect(result.valid).toBe(false);
    });

    it("should reject password without number", () => {
      const result = validatePassword("SecurePassABC");
      expect(result.valid).toBe(false);
    });

    it("should reject password shorter than 8 characters", () => {
      const result = validatePassword("Short1A");
      expect(result.valid).toBe(false);
    });
  });

  describe("validateName", () => {
    it("should validate valid name", () => {
      const result = validateName("John Doe");
      expect(result.valid).toBe(true);
    });

    it("should reject empty name", () => {
      const result = validateName("");
      expect(result.valid).toBe(false);
    });

    it("should respect min length", () => {
      const result = validateName("Jo", 3);
      expect(result.valid).toBe(false);
    });
  });

  describe("validateDomain", () => {
    it("should validate correct domain", () => {
      const result = validateDomain("example.com");
      expect(result.valid).toBe(true);
    });

    it("should validate subdomain", () => {
      const result = validateDomain("mail.example.com");
      expect(result.valid).toBe(true);
    });

    it("should reject invalid domain", () => {
      const result = validateDomain("invalid..domain");
      expect(result.valid).toBe(false);
    });
  });

  describe("validateUrl", () => {
    it("should validate correct URL", () => {
      const result = validateUrl("https://example.com");
      expect(result.valid).toBe(true);
    });

    it("should reject invalid URL", () => {
      const result = validateUrl("not a url");
      expect(result.valid).toBe(false);
    });
  });

  describe("validateConnectionString", () => {
    it("should validate connection string", () => {
      const result = validateConnectionString(
        "postgresql://user:pass@host:5432/db"
      );
      expect(result.valid).toBe(true);
    });

    it("should reject empty string", () => {
      const result = validateConnectionString("");
      expect(result.valid).toBe(false);
    });

    it("should reject too short string", () => {
      const result = validateConnectionString("short");
      expect(result.valid).toBe(false);
    });
  });

  describe("validateLDAPUrl", () => {
    it("should validate LDAP URL", () => {
      const result = validateLDAPUrl("ldap://ldap.example.com:389");
      expect(result.valid).toBe(true);
    });

    it("should validate LDAPS URL", () => {
      const result = validateLDAPUrl("ldaps://ldap.example.com:636");
      expect(result.valid).toBe(true);
    });

    it("should reject non-LDAP URL", () => {
      const result = validateLDAPUrl("https://example.com");
      expect(result.valid).toBe(false);
    });

    it("should reject empty URL", () => {
      const result = validateLDAPUrl("");
      expect(result.valid).toBe(false);
    });
  });

  describe("validateRoleName", () => {
    it("should validate valid role name", () => {
      const result = validateRoleName("Administrator");
      expect(result.valid).toBe(true);
    });

    it("should reject too short name", () => {
      const result = validateRoleName("A");
      expect(result.valid).toBe(false);
    });

    it("should reject too long name", () => {
      const result = validateRoleName("A".repeat(51));
      expect(result.valid).toBe(false);
    });
  });

  describe("validateDateRange", () => {
    it("should validate correct date range", () => {
      const result = validateDateRange("2025-01-01", "2025-12-31");
      expect(result.valid).toBe(true);
    });

    it("should reject if start date is after end date", () => {
      const result = validateDateRange("2025-12-31", "2025-01-01");
      expect(result.valid).toBe(false);
    });

    it("should reject invalid dates", () => {
      const result = validateDateRange("invalid", "2025-12-31");
      expect(result.valid).toBe(false);
    });
  });
});
