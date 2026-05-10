import { describe, it, expect } from "vitest";
import {
  AdminApiError,
  parseErrorResponse,
  getFieldError,
  hasFieldError,
} from "../error-handler";

describe("error-handler", () => {
  describe("AdminApiError", () => {
    it("should create error with message", () => {
      const error = new AdminApiError(400, "Bad request");
      expect(error.message).toBe("Bad request");
      expect(error.status).toBe(400);
    });

    it("should include field errors", () => {
      const details = {
        email: ["Email is required"],
        password: ["Password must be at least 8 characters"],
      };
      const error = new AdminApiError(422, "Validation failed", details);
      expect(error.details).toEqual(details);
      expect(error.getFieldErrors()).toHaveLength(2);
    });

    it("should parse field errors", () => {
      const details = {
        email: ["Invalid email format", "Email already exists"],
      };
      const error = new AdminApiError(422, "Validation failed", details);
      const fieldErrors = error.getFieldErrors();
      expect(fieldErrors).toHaveLength(2);
      expect(fieldErrors[0]).toEqual({
        field: "email",
        message: "Invalid email format",
      });
    });

    it("should identify error types", () => {
      const badRequest = new AdminApiError(400, "Bad request");
      expect(badRequest.isBadRequest()).toBe(true);

      const unauthorized = new AdminApiError(401, "Unauthorized");
      expect(unauthorized.isUnauthorized()).toBe(true);

      const forbidden = new AdminApiError(403, "Forbidden");
      expect(forbidden.isForbidden()).toBe(true);

      const notFound = new AdminApiError(404, "Not found");
      expect(notFound.isNotFound()).toBe(true);

      const serverError = new AdminApiError(500, "Server error");
      expect(serverError.isServerError()).toBe(true);
    });

    it("should provide user-friendly error messages", () => {
      const unauthorized = new AdminApiError(401, "Token expired");
      expect(unauthorized.getErrorMessage()).toContain("session has expired");

      const forbidden = new AdminApiError(403, "Access denied");
      expect(forbidden.getErrorMessage()).toContain("don't have permission");

      const notFound = new AdminApiError(404, "User not found");
      expect(notFound.getErrorMessage()).toContain("not found");

      const serverError = new AdminApiError(500, "Internal error");
      expect(serverError.getErrorMessage()).toContain("unexpected server error");
    });
  });

  describe("parseErrorResponse", () => {
    it("should parse error object with status and data", () => {
      const errorData = {
        status: 422,
        data: {
          message: "Validation failed",
          details: {
            email: ["Invalid email"],
          },
        },
      };
      const error = parseErrorResponse(errorData);
      expect(error).toBeInstanceOf(AdminApiError);
      expect(error.status).toBe(422);
      expect(error.message).toBe("Validation failed");
    });

    it("should return existing AdminApiError unchanged", () => {
      const original = new AdminApiError(400, "Test error");
      const parsed = parseErrorResponse(original);
      expect(parsed).toBe(original);
    });

    it("should handle response.data structure", () => {
      const errorData = {
        response: {
          status: 500,
          data: {
            message: "Server error",
          },
        },
      };
      const error = parseErrorResponse(errorData);
      expect(error.status).toBe(500);
      expect(error.message).toBe("Server error");
    });

    it("should provide default error for unknown format", () => {
      const error = parseErrorResponse({ something: "else" });
      expect(error).toBeInstanceOf(AdminApiError);
      expect(error.status).toBe(500);
    });
  });

  describe("getFieldError", () => {
    it("should return error for field", () => {
      const fieldErrors = [
        { field: "email", message: "Invalid email" },
        { field: "password", message: "Too short" },
      ];
      const error = getFieldError(fieldErrors, "email");
      expect(error).toBe("Invalid email");
    });

    it("should return undefined for missing field", () => {
      const fieldErrors = [
        { field: "email", message: "Invalid email" },
      ];
      const error = getFieldError(fieldErrors, "password");
      expect(error).toBeUndefined();
    });
  });

  describe("hasFieldError", () => {
    it("should detect field error", () => {
      const fieldErrors = [
        { field: "email", message: "Invalid email" },
      ];
      expect(hasFieldError(fieldErrors, "email")).toBe(true);
      expect(hasFieldError(fieldErrors, "password")).toBe(false);
    });
  });
});
