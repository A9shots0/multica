import { z } from "zod";

export const WorkHistoryMatchSchema = z.object({
  id: z.string().default(""),
  provider: z.string().default("unknown"),
  sessionId: z.string().default(""),
  project: z.string().default(""),
  occurredAt: z.string().default(""),
  role: z.string().default("unknown"),
  excerpt: z.string().default(""),
  excerptTruncated: z.boolean().default(false),
  textSha256: z.string().default(""),
  sourceReference: z.string().default(""),
  sourceOrdinal: z.number().default(0),
  sourceOffset: z.number().default(0),
  recordClass: z.string().default("conversation_recollection"),
  assured: z.boolean().default(false),
  accepted: z.boolean().default(false),
  authority: z.string().default("none"),
  matchTypes: z.array(z.string()).default([]),
  exactScore: z.number().optional(),
  semanticScore: z.number().optional(),
  score: z.number().optional(),
}).loose();

export const WorkHistoryGroupSchema = z.object({
  day: z.string().default(""),
  project: z.string().default(""),
  matches: z.array(WorkHistoryMatchSchema).default([]),
}).loose();

export const WorkHistorySearchResponseSchema = z.object({
  schemaVersion: z.string().min(1),
  servedAt: z.string().default(""),
  history: z.object({
    groups: z.array(WorkHistoryGroupSchema).default([]),
    matchCount: z.number().default(0),
    recordClasses: z.array(z.string()).default([]),
    semantic: z.object({
      available: z.boolean().default(false),
      model: z.string().optional(),
      candidateCount: z.number().optional(),
      truncated: z.boolean().optional(),
    }).loose().optional(),
  }).loose(),
}).loose();

export type WorkHistoryMatch = z.infer<typeof WorkHistoryMatchSchema>;
export type WorkHistoryGroup = z.infer<typeof WorkHistoryGroupSchema>;
export type WorkHistorySearchResponse = z.infer<typeof WorkHistorySearchResponseSchema>;

export const EMPTY_WORK_HISTORY_SEARCH_RESPONSE: WorkHistorySearchResponse = {
  schemaVersion: "",
  servedAt: "",
  history: { groups: [], matchCount: 0, recordClasses: [] },
};
