import { describe, it, expect } from "vitest";
import { parseCliTokenResponse } from "./cli-token-response";

describe("parseCliTokenResponse", () => {
  it("returns the token for a well-formed response", () => {
    expect(parseCliTokenResponse({ token: "abc123" })).toEqual({ token: "abc123" });
  });

  it("throws for a missing token field", () => {
    expect(() => parseCliTokenResponse({})).toThrow();
  });

  it("throws for a non-string token", () => {
    expect(() => parseCliTokenResponse({ token: 123 })).toThrow();
  });

  it("throws for an empty token", () => {
    expect(() => parseCliTokenResponse({ token: "" })).toThrow();
  });

  it("throws for a null token", () => {
    expect(() => parseCliTokenResponse({ token: null })).toThrow();
  });

  it("throws for a non-object response", () => {
    expect(() => parseCliTokenResponse(null)).toThrow();
  });
});
