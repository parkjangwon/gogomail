export interface ApiErrorResponse {
  status: number;
  message: string;
  details?: Record<string, string[]>;
  timestamp?: string;
}

export interface FieldError {
  field: string;
  message: string;
}

export class AdminApiError extends Error {
  constructor(
    public status: number,
    public message: string,
    public details?: Record<string, string[]>,
    public timestamp?: string
  ) {
    super(message);
    this.name = "AdminApiError";
  }

  getFieldErrors(): FieldError[] {
    if (!this.details) return [];
    return Object.entries(this.details).flatMap(([field, messages]) =>
      messages.map((message) => ({ field, message }))
    );
  }

  getErrorMessage(): string {
    if (this.status === 401) {
      return "Your session has expired. Please log in again.";
    }
    if (this.status === 403) {
      return "You don't have permission to perform this action.";
    }
    if (this.status === 404) {
      return "The requested resource was not found.";
    }
    if (this.status === 422) {
      const errors = this.getFieldErrors();
      if (errors.length > 0) {
        return errors.map((e) => e.message).join(", ");
      }
      return "The data you submitted is invalid.";
    }
    if (this.status >= 500) {
      return "An unexpected server error occurred. Please try again later.";
    }
    return this.message || "An error occurred";
  }

  isBadRequest(): boolean {
    return this.status === 400 || this.status === 422;
  }

  isUnauthorized(): boolean {
    return this.status === 401;
  }

  isForbidden(): boolean {
    return this.status === 403;
  }

  isNotFound(): boolean {
    return this.status === 404;
  }

  isServerError(): boolean {
    return this.status >= 500;
  }
}

export function parseErrorResponse(error: any): AdminApiError {
  if (error instanceof AdminApiError) {
    return error;
  }

  const status = error?.status || error?.response?.status || 500;
  const data = error?.data || error?.response?.data;
  const message = data?.message || error?.message || "An error occurred";
  const details = data?.details || data?.errors;
  const timestamp = data?.timestamp;

  return new AdminApiError(status, message, details, timestamp);
}

export function getFieldError(
  fieldErrors: FieldError[],
  fieldName: string
): string | undefined {
  return fieldErrors.find((e) => e.field === fieldName)?.message;
}

export function hasFieldError(
  fieldErrors: FieldError[],
  fieldName: string
): boolean {
  return fieldErrors.some((e) => e.field === fieldName);
}

export const ERROR_MESSAGES = {
  NETWORK: "Network error. Please check your connection and try again.",
  TIMEOUT: "Request timed out. Please try again.",
  GENERIC: "An unexpected error occurred. Please try again.",
  SAVE_FAILED: "Failed to save changes. Please try again.",
  DELETE_FAILED: "Failed to delete the item. Please try again.",
  FETCH_FAILED: "Failed to load data. Please try again.",
  SESSION_EXPIRED: "Your session has expired. Please log in again.",
  UNAUTHORIZED: "You are not authorized to perform this action.",
  VALIDATION_ERROR: "Please correct the errors below and try again.",
};
