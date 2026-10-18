package service

import (
	"reflect"
	"testing"
)

func TestChatAttachmentReferencesSurviveMissingClientUploadState(t *testing.T) {
	const workspace = "4e3cdf12-463c-4834-9291-c5ff2e422663"
	const first = "01a11765-9a5e-75ca-a7c9-bf47cb36e251"
	const second = "01a11765-ea4d-7aeb-b6f6-d300970d3f8f"
	content := "![image.png](https://app.example.test/api/attachments/" + first + "/download)\n\n![image.png](/uploads/workspaces/" + workspace + "/" + second + ".png)"
	if got := chatAttachmentReferenceIDs(content, workspace); !reflect.DeepEqual(got, []string{first, second}) {
		t.Fatalf("two visible screenshots became %v", got)
	}
	content += "\n![duplicate](/api/attachments/" + first + "/download)\n`![code](/api/attachments/00000000-0000-0000-0000-000000000001/download)`\n![foreign workspace](/uploads/workspaces/00000000-0000-0000-0000-000000000001/00000000-0000-0000-0000-000000000002.png)"
	if got := chatAttachmentReferenceIDs(content, workspace); !reflect.DeepEqual(got, []string{first, second}) {
		t.Fatalf("duplicate, code or foreign storage reference accepted: %v", got)
	}
}
