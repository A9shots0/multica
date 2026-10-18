package continuitycheckpoint

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDecodeAndValidate(t *testing.T) {
	raw := json.RawMessage(`{"schema_version":1,"package_step":1,"summary":" package one ","completed":[" tests pass "],"next_action":"continue integration","verification":null,"open_risks":[]}`)
	got, err := DecodeAndValidate(raw, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got.Receipt.Summary != "package one" || len(got.SHA256) != 64 || len(got.Canonical) > MaxCanonicalBytes {
		t.Fatalf("validated receipt = %+v digest=%q", got.Receipt, got.SHA256)
	}
	if strings.Contains(string(got.Canonical), "null") {
		t.Fatalf("canonical receipt retained null list: %s", got.Canonical)
	}
}

func TestDecodeAndValidateRejectsDriftAndOversize(t *testing.T) {
	cases := []json.RawMessage{
		json.RawMessage(`{"schema_version":1,"package_step":2,"summary":"x","completed":[],"next_action":"y","verification":[],"open_risks":[]}`),
		json.RawMessage(`{"schema_version":1,"package_step":1,"summary":"x","completed":[],"next_action":"y","verification":[],"open_risks":[],"surprise":true}`),
		json.RawMessage(`{"schema_version":1,"package_step":1,"summary":"","completed":[],"next_action":"y","verification":[],"open_risks":[]}`),
		json.RawMessage(`{"schema_version":1,"package_step":1,"summary":"x\u0000y","completed":["done"],"next_action":"y","verification":[],"open_risks":[]}`),
		json.RawMessage(`{"schema_version":1,"package_step":1,"summary":"x","completed":["bad\nitem"],"next_action":"y","verification":[],"open_risks":[]}`),
	}
	for _, raw := range cases {
		if _, err := DecodeAndValidate(raw, 1); err == nil {
			t.Fatalf("accepted invalid receipt: %s", raw)
		}
	}
	long := strings.Repeat("z", MaxSummaryRunes+1)
	raw, _ := json.Marshal(Receipt{SchemaVersion: 1, PackageStep: 1, Summary: long, NextAction: "y"})
	if _, err := DecodeAndValidate(raw, 1); err == nil {
		t.Fatal("accepted oversized summary")
	}
}
