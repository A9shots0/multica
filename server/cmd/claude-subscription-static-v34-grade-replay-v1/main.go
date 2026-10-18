package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

type ledger struct {
	SchemaVersion string           `json:"schema_version"`
	Status        string           `json:"status"`
	Claims        map[string]claim `json:"claims"`
	GlobalHalt    bool             `json:"global_halt"`
}

type claim struct {
	BindingID      string `json:"binding_id"`
	CaseID         string `json:"case_id"`
	CaseVersion    string `json:"case_version"`
	Status         string `json:"status"`
	Result         string `json:"result"`
	GradeSHA256    string `json:"grade_sha256"`
	ResponseSHA256 string `json:"response_sha256"`
	UsageStatus    string `json:"usage_status"`
}

func main() {
	root := flag.String("artifact-root", "", "immutable v34 evidence root")
	flag.Parse()
	if *root == "" {
		fmt.Fprintln(os.Stderr, "-artifact-root is required")
		os.Exit(2)
	}
	report, err := verify(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	raw, _ := json.MarshalIndent(report, "", "  ")
	fmt.Println(string(raw))
}

func verify(root string) (map[string]any, error) {
	ledgerPath := filepath.Join(root, "ledger.json")
	raw, err := os.ReadFile(ledgerPath)
	if err != nil {
		return nil, err
	}
	var l ledger
	if err := json.Unmarshal(raw, &l); err != nil {
		return nil, fmt.Errorf("invalid ledger: %w", err)
	}
	if l.SchemaVersion != "silver-mediums.claude-subscription-static-ledger.v34" || l.Status != "complete" || l.GlobalHalt || len(l.Claims) != 6 {
		return nil, fmt.Errorf("unexpected v34 ledger state")
	}
	ids := make([]string, 0, len(l.Claims))
	for id := range l.Claims {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	verified := make([]string, 0, len(ids))
	for _, id := range ids {
		c := l.Claims[id]
		if c.BindingID != id || c.Status != "completed" || c.UsageStatus != "exact" || c.Result == "" {
			return nil, fmt.Errorf("invalid completed claim %s", id)
		}
		dir := filepath.Join(root, id)
		response, err := readHashed(filepath.Join(dir, "candidate-response.json"), c.ResponseSHA256)
		if err != nil {
			return nil, fmt.Errorf("%s response: %w", c.CaseID, err)
		}
		gradeRaw, err := readHashed(filepath.Join(dir, "blind-grade.json"), c.GradeSHA256)
		if err != nil {
			return nil, fmt.Errorf("%s grade: %w", c.CaseID, err)
		}
		var sealed agentqualification.BlindGrade
		if err := json.Unmarshal(gradeRaw, &sealed); err != nil {
			return nil, fmt.Errorf("%s sealed grade JSON: %w", c.CaseID, err)
		}
		recomputed, err := agentqualification.GradeBlindVersion(c.CaseID, c.CaseVersion, response)
		if err != nil {
			return nil, fmt.Errorf("%s:%s no longer grades: %w", c.CaseID, c.CaseVersion, err)
		}
		if !equalGrade(recomputed, sealed) || strings.ToLower(recomputed.Verdict) != c.Result {
			return nil, fmt.Errorf("real grade semantic drift for %s:%s", c.CaseID, c.CaseVersion)
		}
		verified = append(verified, c.CaseID+":"+c.CaseVersion+"="+c.Result)
	}
	return map[string]any{
		"status":          "grade_replay_valid",
		"provider_called": false,
		"claims":          len(ids),
		"ledger_sha256":   digest(raw),
		"verified":        verified,
	}, nil
}

func readHashed(path, expected string) ([]byte, error) {
	if expected == "" {
		return nil, fmt.Errorf("expected hash missing")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if digest(raw) != expected {
		return nil, fmt.Errorf("hash drift")
	}
	return raw, nil
}

func digest(raw []byte) string {
	h := sha256.Sum256(raw)
	return hex.EncodeToString(h[:])
}

func equalGrade(a, b agentqualification.BlindGrade) bool {
	return a.SchemaVersion == b.SchemaVersion && a.CaseID == b.CaseID &&
		a.GraderVersion == b.GraderVersion && a.Verdict == b.Verdict &&
		equalStrings(a.PassedChecks, b.PassedChecks) &&
		equalStrings(a.FailedChecks, b.FailedChecks) &&
		equalStrings(a.Disqualifiers, b.Disqualifiers)
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
