// @vitest-environment jsdom
import React from "react";
import { act, cleanup, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, describe, expect, it, vi } from "vitest";
import { emptyLibrary } from "./library";
import { newerLibrary, useConversationLibrary } from "./library-queries";

const api = vi.hoisted(() => ({ getChatLibrary: vi.fn(), changeChatLibrary: vi.fn() }));
vi.mock("../api", () => ({ api }));
afterEach(() => { cleanup(); vi.resetAllMocks(); });

describe("saved conversation organization", () => {
  it("keeps a newer saved change when an older read finishes late", () => {
    const saved = { ...emptyLibrary(), revision: 3, grouping: "custom" as const };
    expect(newerLibrary(saved, { ...emptyLibrary(), revision: 2 })).toBe(saved);
    expect(newerLibrary(saved, { ...emptyLibrary(), revision: 4 }).revision).toBe(4);
  });

  it("retries the failed import without replacing the library on another device", async () => {
    const legacy = { ...emptyLibrary(), groups: [{ id: "old", name: "Ideas", sessionIds: [] }] };
    const saved = { ...legacy, revision: 1 };
    api.getChatLibrary.mockResolvedValue({ ...emptyLibrary(), revision: 0 });
    api.changeChatLibrary.mockRejectedValueOnce(new Error("Offline")).mockImplementationOnce(async () => {
      api.getChatLibrary.mockResolvedValue(saved);
      return saved;
    });
    const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
    const wrapper = ({ children }: { children: React.ReactNode }) => <QueryClientProvider client={client}>{children}</QueryClientProvider>;
    const { result } = renderHook(() => useConversationLibrary("workspace", legacy), { wrapper });
    await waitFor(() => expect(result.current.failed).toBe(true));
    expect(result.current.ready).toBe(false);
    act(() => { result.current.retry(); });
    await waitFor(() => expect(result.current.ready).toBe(true));
    expect(result.current.library.groups).toEqual(legacy.groups);
    expect(api.changeChatLibrary).toHaveBeenCalledTimes(2);
    expect(api.changeChatLibrary).toHaveBeenLastCalledWith({ action: "import", library: legacy });
    client.clear();
  });
});
