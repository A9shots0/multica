package handler

import (
	"encoding/json"
	"testing"
)

func TestTaskCompleteRequestRejectsUnknownContinuityCheckpointField(t *testing.T) {
	var req TaskCompleteRequest
	err := json.Unmarshal([]byte(`{
		"output":"",
		"continuity_checkpoint":{
			"schema_version":1,
			"package_step":1,
			"summary":"done",
			"completed":["tests"],
			"next_action":"continue",
			"verification":[],
			"open_risks":[],
			"unexpected_authority":true
		}
	}`), &req)
	if err == nil {
		t.Fatal("completion request accepted an unknown continuity checkpoint field")
	}
}
