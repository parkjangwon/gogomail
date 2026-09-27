import { describe, it, expect } from "vitest";
import {
  BYTES_PER_GB,
  BYTES_PER_MB,
  BYTES_PER_TB,
  QUOTA_UNITS,
  bestQuotaUnit,
  formatQuotaValue,
  convertQuotaUnit,
  parseQuotaInput,
  validateIntField,
} from "../quota";

describe("quota units", () => {
  it("defines binary unit sizes", () => {
    expect(BYTES_PER_MB).toBe(1048576);
    expect(BYTES_PER_GB).toBe(1024 * 1048576);
    expect(BYTES_PER_TB).toBe(1024 * 1024 * 1048576);
    expect(QUOTA_UNITS).toEqual({
      MB: BYTES_PER_MB,
      GB: BYTES_PER_GB,
      TB: BYTES_PER_TB,
    });
  });
});

describe("bestQuotaUnit", () => {
  it("picks the largest whole unit", () => {
    expect(bestQuotaUnit(10 * BYTES_PER_GB)).toBe("GB");
    expect(bestQuotaUnit(2 * BYTES_PER_TB)).toBe("TB");
    expect(bestQuotaUnit(500 * BYTES_PER_MB)).toBe("MB");
  });

  it("falls back to MB for non-whole GB/TB values", () => {
    expect(bestQuotaUnit(1536 * BYTES_PER_MB)).toBe("MB"); // 1.5 GB
  });
});

describe("formatQuotaValue", () => {
  it("renders whole numbers without decimals", () => {
    expect(formatQuotaValue(10 * BYTES_PER_GB, "GB")).toBe("10");
    expect(formatQuotaValue(10 * BYTES_PER_GB, "MB")).toBe("10240");
  });

  it("trims fractional values to two decimals", () => {
    expect(formatQuotaValue(1536 * BYTES_PER_MB, "GB")).toBe("1.5");
  });
});

describe("unit change round trip (Bug 1: 1024x shrink)", () => {
  // The core contract: changing the display unit alone must NEVER change the
  // stored byte value. Simulate the display-only unit toggle the UI performs.
  it("10 GB -> MB -> GB keeps the byte value identical", () => {
    const originalBytes = 10 * BYTES_PER_GB;

    // Switch GB -> MB: display text changes, bytes do not.
    const bytesAfterMB = convertQuotaUnit(originalBytes);
    expect(bytesAfterMB).toBe(originalBytes);
    expect(formatQuotaValue(bytesAfterMB, "MB")).toBe("10240");

    // Switch MB -> GB: still the same bytes.
    const bytesAfterGB = convertQuotaUnit(bytesAfterMB);
    expect(bytesAfterGB).toBe(originalBytes);
    expect(formatQuotaValue(bytesAfterGB, "GB")).toBe("10");
  });

  it("does not shrink by 1024x when re-parsing the displayed number", () => {
    // Regression guard for the old bug: parsing "10" (a GB display) under MB
    // would have produced 10 MB. parseQuotaInput must be given the CURRENT
    // unit; here we assert the byte-preserving path is used instead.
    const originalBytes = 10 * BYTES_PER_GB;
    const displayed = formatQuotaValue(originalBytes, "GB"); // "10"
    // Correct: re-render same bytes in MB.
    expect(formatQuotaValue(convertQuotaUnit(originalBytes), "MB")).toBe("10240");
    // Buggy behaviour would have been parseQuotaInput(displayed, 'MB') = 10 MB.
    const buggy = parseQuotaInput(displayed, "MB");
    expect(buggy.bytes).toBe(10 * BYTES_PER_MB);
    expect(buggy.bytes).not.toBe(originalBytes); // proves why the old path corrupts
  });
});

describe("parseQuotaInput (Bug 2: invalid becomes unlimited)", () => {
  it("treats empty/whitespace as explicit unlimited", () => {
    expect(parseQuotaInput("", "GB")).toEqual({ valid: true, bytes: null, error: "" });
    expect(parseQuotaInput("   ", "GB")).toEqual({ valid: true, bytes: null, error: "" });
  });

  it("rejects non-numeric input instead of coercing to unlimited", () => {
    const r = parseQuotaInput("abc", "GB");
    expect(r.valid).toBe(false);
    expect(r.bytes).toBeNull();
    expect(r.error).toBe("invalid_number");
  });

  it("rejects negative values", () => {
    const r = parseQuotaInput("-5", "GB");
    expect(r.valid).toBe(false);
    expect(r.error).toBe("negative");
  });

  it("rejects fractional values by default", () => {
    const r = parseQuotaInput("1.5", "GB");
    expect(r.valid).toBe(false);
    expect(r.error).toBe("fraction");
  });

  it("allows fractional values when opted in and rounds to bytes", () => {
    const r = parseQuotaInput("1.5", "GB", { allowFractional: true });
    expect(r.valid).toBe(true);
    expect(r.bytes).toBe(Math.round(1.5 * BYTES_PER_GB));
  });

  it("converts a valid whole number to bytes in the given unit", () => {
    expect(parseQuotaInput("10", "GB").bytes).toBe(10 * BYTES_PER_GB);
    expect(parseQuotaInput("512", "MB").bytes).toBe(512 * BYTES_PER_MB);
    expect(parseQuotaInput("0", "GB").bytes).toBe(0);
  });
});

describe("validateIntField (Bug 3: number fields without validation)", () => {
  it("rejects blank input", () => {
    expect(validateIntField("").valid).toBe(false);
    expect(validateIntField("").error).toBe("required");
  });

  it("rejects non-integers and negatives", () => {
    expect(validateIntField("abc").error).toBe("invalid_integer");
    expect(validateIntField("1.5").error).toBe("invalid_integer");
    expect(validateIntField("-5", { min: 0 }).error).toBe("below_min");
  });

  it("enforces min/max range", () => {
    expect(validateIntField("2", { min: 4, max: 128 }).error).toBe("below_min");
    expect(validateIntField("200", { min: 4, max: 128 }).error).toBe("above_max");
    const ok = validateIntField("8", { min: 4, max: 128 });
    expect(ok.valid).toBe(true);
    expect(ok.value).toBe(8);
  });
});
