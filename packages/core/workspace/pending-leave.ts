/**
 * Registry of workspace IDs whose LEAVE was initiated by THIS client.
 *
 * Marked in useLeaveWorkspace.onMutate. The realtime `member:removed`
 * handler checks it and no-ops for self-initiated leaves: the mutation
 * flow owns navigation (which runs after the POST resolves on success),
 * and letting the handler react too would race that flow's navigation
 * with its own full-page relocate.
 *
 * Lifted when the POST fails — the user is still a member, so a later
 * external removal of the same ID must be handled normally — and when this
 * user is added back to the workspace (`member:added`), which ends the
 * earlier leave. Until then the mark suppresses the WS echo of our own
 * leave no matter when it arrives.
 *
 * Module scope rather than React state because the realtime handler runs
 * outside the component tree. Per-tab by construction: other tabs/devices
 * have an empty registry, so their handlers process the event normally.
 */
const pendingLeaves = new Set<string>();

export function markWorkspaceLeavePending(workspaceId: string) {
  pendingLeaves.add(workspaceId);
}

export function unmarkWorkspaceLeavePending(workspaceId: string) {
  pendingLeaves.delete(workspaceId);
}

/** True if this client initiated a LEAVE for the workspace. */
export function isWorkspaceLeavePending(workspaceId: string): boolean {
  return pendingLeaves.has(workspaceId);
}
