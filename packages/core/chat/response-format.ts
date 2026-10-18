/** Per-turn presentation preference; never an execution or approval mode. */
export const CHAT_RESPONSE_FORMATS = ["plan", "compare", "checklist", "diagram"] as const;
export type ChatResponseFormat = (typeof CHAT_RESPONSE_FORMATS)[number];

export function isChatResponseFormat(value: unknown): value is ChatResponseFormat {
  return CHAT_RESPONSE_FORMATS.some((format) => format === value);
}
