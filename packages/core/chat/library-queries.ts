import { useEffect, useRef } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "../api";
import { emptyLibrary, type ConversationLibrary, type ConversationLibraryChange, type SavedConversationLibrary } from "./library";

export function newerLibrary(previous: unknown, incoming: SavedConversationLibrary): SavedConversationLibrary {
  const current = previous as SavedConversationLibrary | undefined;
  return current && current.revision > incoming.revision ? current : incoming;
}

export function useConversationLibrary(wsId: string, legacy: ConversationLibrary) {
  const client = useQueryClient();
  const key = ["chat-library", wsId] as const;
  const query = useQuery({
    queryKey: key, queryFn: () => api.getChatLibrary(), enabled: !!wsId,
    refetchOnWindowFocus: "always", refetchOnReconnect: "always", refetchInterval: 15000,
    structuralSharing: (old, incoming) => newerLibrary(old, incoming as SavedConversationLibrary),
  });
  const mutation = useMutation({
    scope: { id: `chat-library:${wsId}` },
    mutationFn: (change: ConversationLibraryChange) => api.changeChatLibrary(change),
    onSuccess: (saved) => client.setQueryData<SavedConversationLibrary>(key, current => newerLibrary(current, saved)),
    onSettled: () => client.invalidateQueries({ queryKey: key }),
  });
  const imported = useRef<string | null>(null);
  const { mutate, isPending } = mutation;
  useEffect(() => {
    if (query.data?.revision === 0 && imported.current !== wsId && !isPending) {
      imported.current = wsId;
      mutate({ action: "import", library: legacy });
    }
  }, [query.data?.revision, wsId, legacy, isPending, mutate]);
  // A failed import remains available for an explicit retry; never overwrite a
  // library already saved by another device with an old local snapshot.
  const retry = () => mutation.isError && mutation.variables
    ? mutation.mutate(mutation.variables)
    : query.data?.revision === 0
    ? mutation.mutate({ action: "import", library: legacy })
    : void query.refetch();
  return { library: query.data ?? emptyLibrary(), change: mutation.mutateAsync, retry,
    pending: mutation.isPending, failed: mutation.isError || query.isError, ready: !!query.data && query.data.revision > 0 };
}
