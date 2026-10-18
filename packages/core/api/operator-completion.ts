import { z } from "zod";

export interface OperatorCompletionRequest {
  expectedPlanRevision: number;
  expectedPictureRevision: number;
  reviewTaskId: string;
  evidenceCommentId: string;
  evidenceCommentRevision: number;
  candidateSha256: string;
  reason: string;
}

export const OperatorCompletionReceiptSchema = z.object({
  completion_id: z.string().uuid(),
  picture_revision: z.number().int().positive(),
  completion_mode: z.literal("operator_assisted"),
  replay: z.boolean(),
}).transform((receipt) => ({
  completionId: receipt.completion_id,
  pictureRevision: receipt.picture_revision,
  completionMode: receipt.completion_mode,
  replay: receipt.replay,
}));

export type OperatorCompletionReceipt = z.infer<typeof OperatorCompletionReceiptSchema>;
