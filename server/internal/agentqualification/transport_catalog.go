package agentqualification

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
)

const TransportCatalogSchemaVersion = "silver-mediums.qualification-transport-catalog.v2"

type QualificationTransport struct {
	ID                   string   `json:"id"`
	BillingBoundary      string   `json:"billing_boundary"`
	AccountLanes         []string `json:"account_lanes"`
	PreventiveToolPolicy string   `json:"preventive_tool_policy"`
	UsageReceipt         string   `json:"usage_receipt"`
	Status               string   `json:"status"`
	Reasons              []string `json:"reasons"`
}

type TransportCatalog struct {
	SchemaVersion string                   `json:"schema_version"`
	CatalogID     string                   `json:"catalog_id"`
	Transports    []QualificationTransport `json:"transports"`
}

//go:embed transport_catalog_v2.json
var transportCatalogJSON []byte

func CurrentTransportCatalog() (TransportCatalog, error) {
	var catalog TransportCatalog
	if err := json.Unmarshal(transportCatalogJSON, &catalog); err != nil {
		return catalog, fmt.Errorf("decode transport catalog: %w", err)
	}
	if err := ValidateTransportCatalog(catalog); err != nil {
		return TransportCatalog{}, err
	}
	return catalog, nil
}

func ValidateTransportCatalog(catalog TransportCatalog) error {
	if (catalog.SchemaVersion != TransportCatalogSchemaVersion && catalog.SchemaVersion != "silver-mediums.qualification-transport-catalog.v1") || strings.TrimSpace(catalog.CatalogID) == "" {
		return fmt.Errorf("transport catalog identity is incomplete")
	}
	seen := map[string]struct{}{}
	for _, transport := range catalog.Transports {
		if strings.TrimSpace(transport.ID) == "" || strings.TrimSpace(transport.BillingBoundary) == "" || len(transport.AccountLanes) == 0 || strings.TrimSpace(transport.PreventiveToolPolicy) == "" || strings.TrimSpace(transport.UsageReceipt) == "" || len(transport.Reasons) == 0 {
			return fmt.Errorf("transport entry is incomplete")
		}
		if _, duplicate := seen[transport.ID]; duplicate {
			return fmt.Errorf("duplicate transport %s", transport.ID)
		}
		seen[transport.ID] = struct{}{}
		if transport.Status != "held" && transport.Status != "utility_only" && transport.Status != "external_only" && transport.Status != "qualified" {
			return fmt.Errorf("transport %s has invalid status", transport.ID)
		}
		if transport.Status == "qualified" && (transport.PreventiveToolPolicy == "unknown" || transport.PreventiveToolPolicy == "not_proven" || transport.UsageReceipt == "unknown") {
			return fmt.Errorf("transport %s is fail-open", transport.ID)
		}
	}
	if len(seen) != 6 {
		return fmt.Errorf("transport catalog must preserve all six owner lanes")
	}
	return nil
}

func (catalog TransportCatalog) QualifiedTransports() []QualificationTransport {
	result := []QualificationTransport{}
	for _, transport := range catalog.Transports {
		if transport.Status == "qualified" {
			result = append(result, transport)
		}
	}
	return result
}
