import { z } from "zod";
import type { ChatWorkIssue } from "./work";

const reasonSchema = z.union([z.string(), z.object({ description: z.string().optional(), message: z.string().optional(), reason: z.string().optional(), code: z.string().optional() })]);
const dependencySchema = z.object({ id: z.string(), actor_id: z.string().optional(), actor_type: z.string().optional(), wait_reason: reasonSchema.optional() });
const pictureSchema = z.object({
  contract_version: z.union([z.literal(2), z.literal(3)]),
  owner_summary_schema_version: z.literal(1),
  current_milestone: z.string().optional(),
  active_actor: z.object({ actor_id: z.string().optional(), actor_type: z.string().optional() }).optional(),
  wait_reason: reasonSchema.optional(),
  unresolved_dependencies: z.array(dependencySchema).optional(),
  next_automatic_action: z.object({ expected_event: z.object({ type: z.string().optional() }).optional() }).optional(),
});

function reasonText(reason: z.infer<typeof reasonSchema> | undefined) {
  return (typeof reason === "string" ? reason : reason?.description || reason?.message || reason?.reason)?.trim() || null;
}

/** Show recorded facts only. Unknown shapes do not become invented progress. */
export function chatWorkDetails(issue: ChatWorkIssue) {
  const parsed = pictureSchema.safeParse(issue.picture);
  if (!parsed.success || issue.summary.current_picture_revision < 1) return null;
  const picture = parsed.data;
  return {
    milestone: picture.current_milestone,
    actor: picture.active_actor,
    waitReason: reasonText(picture.wait_reason),
    nextEvent: picture.next_automatic_action?.expected_event?.type,
    dependencies: (picture.unresolved_dependencies ?? []).map(dependency => ({
      id: dependency.id, actorId: dependency.actor_id, actorType: dependency.actor_type,
      reason: reasonText(dependency.wait_reason),
    })),
  };
}
