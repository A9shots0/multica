package agentqualification

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

type roleQualificationProviderV4Fake struct {
	t        *testing.T
	calls    int
	badRoute bool
}

func (f *roleQualificationProviderV4Fake) Call(_ context.Context, c RoleQualificationExecutionCellV3) (json.RawMessage, error) {
	f.calls++
	r := passingReceiptV3(f.t, c)
	if f.badRoute {
		var p RoleQualificationRawProviderResponseV3
		_ = strictJSON(r.RawResponse, &p)
		p.ActualModel = "fallback"
		r.RawResponse, _ = json.Marshal(p)
	}
	return r.RawResponse, nil
}
func TestRoleQualificationExecutionV4ExactClassBatchOnly(t *testing.T) {
	plan, _ := BuildRoleQualificationExecutionPlanV3()
	classes := map[string]bool{}
	for _, c := range plan.Cells {
		classes[c.ConfigurationClass.ClassID] = true
	}
	if len(classes) != 3 {
		t.Fatal(classes)
	}
	for class := range classes {
		cells, err := RoleQualificationClassCellsV4(class)
		if err != nil || len(cells) != 12 {
			t.Fatalf("%s %d %v", class, len(cells), err)
		}
	}
	if _, err := RoleQualificationClassCellsV4("all"); err == nil {
		t.Fatal("implicit all-model batch accepted")
	}
}
func TestRoleQualificationExecutionV4FreshPreflightBindsExactRoute(t *testing.T) {
	plan, _ := BuildRoleQualificationExecutionPlanV3()
	cell := plan.Cells[0]
	now := time.Now().UTC()
	p := preflightV4Fixture(cell, now)
	if err := ValidateRoleQualificationPreflightV4(p, cell.ConfigurationClass.ClassID, now); err != nil {
		t.Fatal(err)
	}
	p.RequestedModel = "fallback"
	if err := ValidateRoleQualificationPreflightV4(p, cell.ConfigurationClass.ClassID, now); err == nil {
		t.Fatal("route drift accepted")
	}
	p = preflightV4Fixture(cell, now.Add(-11*time.Minute))
	if err := ValidateRoleQualificationPreflightV4(p, cell.ConfigurationClass.ClassID, now); err == nil {
		t.Fatal("stale preflight accepted")
	}
}
func TestRoleQualificationExecutionV4FakeBatchUsesOneCallAndStopsPairLocally(t *testing.T) {
	plan, _ := BuildRoleQualificationExecutionPlanV3()
	class := plan.Cells[0].ConfigurationClass.ClassID
	root := t.TempDir()
	now := time.Now().UTC()
	p := preflightV4Fixture(plan.Cells[0], now)
	if err := WriteRoleQualificationPreflightV4(root, p); err != nil {
		t.Fatal(err)
	}
	provider := &roleQualificationProviderV4Fake{t: t}
	replay, err := ExecuteRoleQualificationBatchV4(context.Background(), root, class, now, provider)
	if err != nil {
		t.Fatal(err)
	}
	if provider.calls != 12 || replay.Observations != 12 || replay.Totals.Calls != 12 || replay.GlobalHalt {
		t.Fatalf("calls=%d replay=%+v", provider.calls, replay)
	}
}
func TestRoleQualificationExecutionV4TransportFailureLeavesStrandedGlobalHalt(t *testing.T) {
	plan, _ := BuildRoleQualificationExecutionPlanV3()
	class := plan.Cells[0].ConfigurationClass.ClassID
	root := t.TempDir()
	now := time.Now().UTC()
	_ = WriteRoleQualificationPreflightV4(root, preflightV4Fixture(plan.Cells[0], now))
	provider := roleQualificationProviderV4Func(func(context.Context, RoleQualificationExecutionCellV3) (json.RawMessage, error) {
		return nil, context.DeadlineExceeded
	})
	replay, err := ExecuteRoleQualificationBatchV4(context.Background(), root, class, now, provider)
	if err == nil || !replay.GlobalHalt || replay.GlobalHaltClass != "evidence_transport_uncertainty" {
		t.Fatalf("replay=%+v err=%v", replay, err)
	}
}
func TestRoleQualificationExecutionV4BadActualRouteFailsClosed(t *testing.T) {
	plan, _ := BuildRoleQualificationExecutionPlanV3()
	class := plan.Cells[0].ConfigurationClass.ClassID
	root := t.TempDir()
	now := time.Now().UTC()
	_ = WriteRoleQualificationPreflightV4(root, preflightV4Fixture(plan.Cells[0], now))
	provider := &roleQualificationProviderV4Fake{t: t, badRoute: true}
	replay, err := ExecuteRoleQualificationBatchV4(context.Background(), root, class, now, provider)
	if err == nil || !replay.GlobalHalt || !strings.Contains(replay.GlobalHaltClass, "evidence") {
		t.Fatalf("replay=%+v err=%v", replay, err)
	}
}

type roleQualificationProviderV4Func func(context.Context, RoleQualificationExecutionCellV3) (json.RawMessage, error)

func (f roleQualificationProviderV4Func) Call(ctx context.Context, c RoleQualificationExecutionCellV3) (json.RawMessage, error) {
	return f(ctx, c)
}
func preflightV4Fixture(cell RoleQualificationExecutionCellV3, now time.Time) RoleQualificationPreflightV4 {
	return RoleQualificationPreflightV4{"silver-mediums.role-qualification-preflight.v4", RoleQualificationExecutionFreezeV4SHA256(), RoleQualificationExecutionFreezeV3SHA256(), cell.ConfigurationClass.ClassID, cell.ConfigurationClass.Model, cell.ConfigurationClass.ServingProvider, cell.ConfigurationClass.Effort, transportForRoleQualificationCellV3(cell), "fixture-runtime", "C:/fixture/runtime.exe", strings.Repeat("a", 64), strings.Repeat("b", 64), true, true, "authenticated", 0, now.Format(time.RFC3339Nano), true}
}
