import { describe, it, expect, vi, beforeEach } from "vitest";
import { ApiClient } from "./client";

function makeClient(responseBody: unknown, status = 200): ApiClient {
  const client = new ApiClient("http://localhost");
  client.setToken("test-token");
  vi.spyOn(global, "fetch").mockResolvedValue({
    ok: status >= 200 && status < 300,
    status,
    statusText: "OK",
    json: () => Promise.resolve(responseBody),
    headers: new Headers(),
  } as Response);
  return client;
}

beforeEach(() => {
  vi.restoreAllMocks();
});

const methods = [
  {
    name: "getUnreadInboxCount",
    call: (client: ApiClient) => client.getUnreadInboxCount(),
  },
  {
    name: "markAllInboxRead",
    call: (client: ApiClient) => client.markAllInboxRead(),
  },
  {
    name: "archiveAllInbox",
    call: (client: ApiClient) => client.archiveAllInbox(),
  },
  {
    name: "archiveAllReadInbox",
    call: (client: ApiClient) => client.archiveAllReadInbox(),
  },
] as const;

for (const { name, call } of methods) {
  describe(name, () => {
    it("returns count from a well-formed response", async () => {
      const client = makeClient({ count: 7 });
      const result = await call(client);
      expect(result).toEqual({ count: 7 });
    });

    it("falls back to count=0 when count is a string", async () => {
      const client = makeClient({ count: "not-a-number" });
      const result = await call(client);
      expect(result).toEqual({ count: 0 });
    });

    it("falls back to count=0 when count field is missing", async () => {
      const client = makeClient({});
      const result = await call(client);
      expect(result).toEqual({ count: 0 });
    });

    it("falls back to count=0 when response is not an object", async () => {
      const client = makeClient("malformed");
      const result = await call(client);
      expect(result).toEqual({ count: 0 });
    });
  });
}
