package migrations

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCompanyTopologyMappingsRemainExplicitAndEvidenceBound(t *testing.T) {
	service := readWorkflowV2File(t, filepath.Join(realMigrationsDir(t), "632_company_service_topology_mapping.up.sql"))
	for _, required := range []string{"system_node_ids", "journey_ids", "array_position"} {
		if !strings.Contains(service, required) {
			t.Fatalf("632 service topology migration is missing %q", required)
		}
	}

	work := readWorkflowV2File(t, filepath.Join(realMigrationsDir(t), "633_company_work_topology_binding.up.sql"))
	for _, required := range []string{
		"PRIMARY KEY (workspace_id, issue_id)", "graph_version", "system_node_ids", "journey_ids",
		"cardinality(system_node_ids) + cardinality(journey_ids) > 0", "authority IN ('owner', 'cited')",
		"authority <> 'cited' OR cardinality(evidence_refs) > 0", "expires_at > observed_at",
	} {
		if !strings.Contains(work, required) {
			t.Fatalf("633 work topology migration is missing %q", required)
		}
	}
}
