/**
 * @vitest-environment jsdom
 */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { createElement } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { setApiInstance } from "../api";
import type { ApiClient } from "../api/client";
import type { Workspace } from "../types";
import { useLeaveWorkspace } from "./mutations";
import { workspaceKeys } from "./queries";
import {
  isWorkspaceLeavePending,
  unmarkWorkspaceLeavePending,
} from "./pending-leave";

function createWrapper(qc: QueryClient) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return createElement(QueryClientProvider, { client: qc }, children);
  };
}

const makeWorkspace = (id: string, slug: string): Workspace => ({
  id,
  name: slug,
  slug,
  description: null,
  context: null,
  settings: {},
  repos: [],
  issue_prefix: "TST",
  avatar_url: null,
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z",
});

describe("useLeaveWorkspace", () => {
  let qc: QueryClient;
  let leaveWorkspace: ReturnType<typeof vi.fn<(id: string) => Promise<void>>>;

  const seedList = () => {
    qc.setQueryData<Workspace[]>(workspaceKeys.list(), [
      makeWorkspace("ws-1", "stay"),
      makeWorkspace("ws-2", "leave-me"),
    ]);
  };

  beforeEach(() => {
    qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    leaveWorkspace = vi.fn().mockResolvedValue(undefined);
    setApiInstance({ leaveWorkspace } as unknown as ApiClient);
  });

  afterEach(() => {
    qc.clear();
    // pending-leave is module state; reset so tests stay independent.
    unmarkWorkspaceLeavePending("ws-2");
    vi.restoreAllMocks();
  });

  it("marks the workspace as leave-pending while the POST is in flight", async () => {
    seedList();
    let resolveLeave!: () => void;
    leaveWorkspace.mockReturnValue(
      new Promise<void>((resolve) => {
        resolveLeave = resolve;
      }),
    );

    const { result } = renderHook(() => useLeaveWorkspace(), {
      wrapper: createWrapper(qc),
    });

    let mutationDone: Promise<void>;
    await act(async () => {
      mutationDone = result.current.mutateAsync("ws-2");
      await Promise.resolve();
    });

    expect(isWorkspaceLeavePending("ws-2")).toBe(true);

    await act(async () => {
      resolveLeave();
      await mutationDone;
    });
  });

  it("keeps the self-initiated mark after a successful leave (suppresses the WS echo)", async () => {
    seedList();
    const { result } = renderHook(() => useLeaveWorkspace(), {
      wrapper: createWrapper(qc),
    });

    await act(async () => {
      await result.current.mutateAsync("ws-2");
    });

    // The mark must survive success: the `member:removed` WS echo can arrive
    // at any time after the mutation resolves, and the realtime handler must
    // no-op for it rather than triggering a second navigation.
    expect(isWorkspaceLeavePending("ws-2")).toBe(true);
  });

  it("lifts the mark after a failed leave so external removals are handled normally", async () => {
    seedList();
    leaveWorkspace.mockRejectedValue(new Error("server error"));

    const { result } = renderHook(() => useLeaveWorkspace(), {
      wrapper: createWrapper(qc),
    });

    await act(async () => {
      await expect(result.current.mutateAsync("ws-2")).rejects.toThrow("server error");
    });

    // The user is still a member: a later `member:removed` from an external
    // removal must NOT be suppressed by a stale pending mark.
    expect(isWorkspaceLeavePending("ws-2")).toBe(false);
  });

  it("invalidates the workspace list on a successful leave", async () => {
    seedList();
    const { result } = renderHook(() => useLeaveWorkspace(), {
      wrapper: createWrapper(qc),
    });

    await act(async () => {
      await result.current.mutateAsync("ws-2");
    });

    expect(qc.getQueryState(workspaceKeys.list())?.isInvalidated).toBe(true);
  });

  it("invalidates the workspace list even when the leave fails", async () => {
    seedList();
    leaveWorkspace.mockRejectedValue(new Error("network error"));

    const { result } = renderHook(() => useLeaveWorkspace(), {
      wrapper: createWrapper(qc),
    });

    await act(async () => {
      await expect(result.current.mutateAsync("ws-2")).rejects.toThrow("network error");
    });

    // Settled-path invalidation ensures the list reflects server truth even
    // after an ambiguous failure (server may have committed before client lost
    // the response).
    expect(qc.getQueryState(workspaceKeys.list())?.isInvalidated).toBe(true);
  });
});
