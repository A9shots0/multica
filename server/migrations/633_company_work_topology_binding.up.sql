CREATE TABLE company_work_topology_binding (
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    issue_id UUID NOT NULL REFERENCES issue(id) ON DELETE CASCADE,
    graph_version TEXT NOT NULL CHECK (length(btrim(graph_version)) BETWEEN 1 AND 256),
    system_node_ids TEXT[] NOT NULL DEFAULT '{}',
    journey_ids TEXT[] NOT NULL DEFAULT '{}',
    authority TEXT NOT NULL CHECK (authority IN ('owner', 'cited')),
    evidence_refs TEXT[] NOT NULL DEFAULT '{}',
    observed_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ,
    revision BIGINT NOT NULL DEFAULT 1 CHECK (revision >= 1),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, issue_id),
    CHECK (cardinality(system_node_ids) + cardinality(journey_ids) > 0),
    CHECK (array_position(system_node_ids, '') IS NULL),
    CHECK (array_position(journey_ids, '') IS NULL),
    CHECK (array_position(evidence_refs, '') IS NULL),
    CHECK (authority <> 'cited' OR cardinality(evidence_refs) > 0),
    CHECK (expires_at IS NULL OR expires_at > observed_at)
);

CREATE INDEX idx_company_work_topology_binding_workspace_updated
    ON company_work_topology_binding (workspace_id, updated_at DESC, issue_id);
