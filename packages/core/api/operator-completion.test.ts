// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiClient } from "./client";

afterEach(() => vi.unstubAllGlobals());

const request = {
  expectedPlanRevision: 3,
  expectedPictureRevision: 1,
  reviewTaskId: "review-id",
  evidenceCommentId: "comment-id",
  evidenceCommentRevision: 2,
  candidateSha256: "a".repeat(64),
  reason: "Accepted the independently reviewed publication with operator assistance.",
};
const receipt = {
  completion_id: "11111111-1111-4111-8111-111111111111",
  picture_revision: 2,
  completion_mode: "operator_assisted",
  replay: false,
};

describe("operator-assisted completion boundary", () => {
  it("sends exact evidence and parses the explicit assisted receipt", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify(receipt)));
    vi.stubGlobal("fetch", fetchMock);
    const result = await new ApiClient("https://api.example.test").completeIssueWithOperatorAssistance("SILV/38", request);
    expect(result).toEqual({ completionId: receipt.completion_id, pictureRevision: 2, completionMode: "operator_assisted", replay: false });
    const [url, options] = fetchMock.mock.calls[0]!;
    expect(url).toBe("https://api.example.test/api/issues/SILV%2F38/operator-completion");
    expect(JSON.parse(options.body)).toEqual({
      expected_plan_revision: 3, expected_picture_revision: 1,
      review_task_id: "review-id", evidence_comment_id: "comment-id", evidence_comment_revision: 2,
      candidate_sha256: request.candidateSha256, reason: request.reason,
    });
  });

  it.each([null, {}, { ...receipt, picture_revision: 0 }, { ...receipt, completion_mode: "automatic" }, { ...receipt, replay: "false" }])("rejects a malformed completion receipt: %j", async (malformed) => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify(malformed))));
    await expect(new ApiClient("https://api.example.test").completeIssueWithOperatorAssistance("SILV-38", request)).rejects.toThrow("malformed receipt");
  });
});
