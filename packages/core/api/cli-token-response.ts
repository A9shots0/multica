import { z } from "zod";
import { parseWithFallback } from "./schema";

export const CliTokenResponseSchema = z.object({
  token: z.string().min(1),
});

export function parseCliTokenResponse(raw: unknown): { token: string } {
  const result = parseWithFallback<{ token: string } | null>(
    raw,
    CliTokenResponseSchema,
    null,
    { endpoint: "POST /api/cli-token" },
  );
  if (!result) {
    throw new Error("POST /api/cli-token returned a malformed response");
  }
  return result;
}
