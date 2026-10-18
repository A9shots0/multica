package handler

import (
	"context"
	"net/http"
	"time"
)

type intelligenceProducerEvent struct {
	Producer  string `json:"producer"`
	EventType string `json:"event_type"`
	Count     int64  `json:"count"`
	Latest    string `json:"latest"`
	Earliest  string `json:"earliest"`
}

type intelligenceSummaryResponse struct {
	Configured bool                        `json:"configured"`
	Available  bool                        `json:"available"`
	Events     []intelligenceProducerEvent `json:"events"`
	TotalCount int64                       `json:"total_count"`
}

// GetIntelligenceSummary reads a live, read-only count/freshness breakdown
// straight off Silver Mediums Intelligence's append-only source_events
// ledger — a separate Postgres instance on a different host (see
// projects/intelligence/CONTEXT.md in the knowledge hub for the full system:
// what it ingests, from where, and what it deliberately does not claim yet).
//
// Deliberately a summary, not a browser: this groups by (producer,
// event_type) rather than exposing individual rows or their JSON payloads.
// Intelligence's own architecture reserves "human-facing surface" for CRM;
// this page is the owner's dashboard view of what the ledger currently
// holds, not a second consumer of its row-level data.
func (h *Handler) GetIntelligenceSummary(w http.ResponseWriter, r *http.Request) {
	if h.IntelligenceDB == nil {
		writeJSON(w, http.StatusOK, intelligenceSummaryResponse{Configured: false})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	rows, err := h.IntelligenceDB.Query(ctx, `
		SELECT producer, event_type, count(*), max(received_at), min(received_at)
		FROM source_events
		GROUP BY producer, event_type
		ORDER BY producer, event_type
	`)
	if err != nil {
		writeJSON(w, http.StatusOK, intelligenceSummaryResponse{Configured: true, Available: false})
		return
	}
	defer rows.Close()

	var events []intelligenceProducerEvent
	var total int64
	for rows.Next() {
		var e intelligenceProducerEvent
		var latest, earliest time.Time
		if err := rows.Scan(&e.Producer, &e.EventType, &e.Count, &latest, &earliest); err != nil {
			writeJSON(w, http.StatusOK, intelligenceSummaryResponse{Configured: true, Available: false})
			return
		}
		e.Latest = latest.UTC().Format(time.RFC3339)
		e.Earliest = earliest.UTC().Format(time.RFC3339)
		events = append(events, e)
		total += e.Count
	}
	if rows.Err() != nil {
		writeJSON(w, http.StatusOK, intelligenceSummaryResponse{Configured: true, Available: false})
		return
	}

	writeJSON(w, http.StatusOK, intelligenceSummaryResponse{
		Configured: true,
		Available:  true,
		Events:     events,
		TotalCount: total,
	})
}

type intelligenceStatusBreakdown struct {
	Status string `json:"status"`
	Count  int64  `json:"count"`
}

type intelligenceCollectorHealth struct {
	Producer    string `json:"producer"`
	Collector   string `json:"collector"`
	ObservedAt  string `json:"observed_at"`
	Accepted    int64  `json:"accepted"`
	Rejected    int64  `json:"rejected"`
	Quarantined int64  `json:"quarantined"`
	HasErrors   bool   `json:"has_errors"`
}

type intelligenceBusinessSummaryResponse struct {
	Configured bool `json:"configured"`
	Available  bool `json:"available"`

	// Latest per-account follower snapshot, summed — "how big is our reach
	// right now", not "how many snapshot rows have we recorded".
	TrackedSocialAccounts int64 `json:"tracked_social_accounts"`
	TotalFollowers        int64 `json:"total_followers"`

	ClientsByStatus        []intelligenceStatusBreakdown `json:"clients_by_status"`
	SocialAccountsByStatus []intelligenceStatusBreakdown `json:"social_accounts_by_status"`

	// Sum of new_subs_gained across clients over the last 7 observed days,
	// deduped to the latest ingested row per (client, day).
	NewSubscribers7d int64 `json:"new_subscribers_7d"`

	// Latest collection.run per producer — "is data still flowing in".
	CollectionHealth []intelligenceCollectorHealth `json:"collection_health"`
}

// GetIntelligenceBusinessSummary answers "what do these numbers mean for the
// business", not "what does the ledger hold" (that's GetIntelligenceSummary,
// a row-count/freshness view of the raw event log). It reads the actual
// event payloads and derives owner-relevant figures: current total
// followers (not snapshot-row count), active client/account counts, recent
// subscriber growth, and whether each producer's last collection run
// actually succeeded. See knowledge hub CONTEXT.md files for what each
// producer (canopy/crm/linkhub) is and what it feeds.
func (h *Handler) GetIntelligenceBusinessSummary(w http.ResponseWriter, r *http.Request) {
	if h.IntelligenceDB == nil {
		writeJSON(w, http.StatusOK, intelligenceBusinessSummaryResponse{Configured: false})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	fail := func() {
		writeJSON(w, http.StatusOK, intelligenceBusinessSummaryResponse{Configured: true, Available: false})
	}

	resp := intelligenceBusinessSummaryResponse{Configured: true, Available: true}

	err := h.IntelligenceDB.QueryRow(ctx, `
		SELECT count(*), coalesce(sum(followers), 0) FROM (
			SELECT DISTINCT ON (payload->>'canopy_account_id')
				(payload->>'followers')::bigint AS followers
			FROM source_events
			WHERE producer = 'canopy' AND event_type = 'follower.snapshot'
			ORDER BY payload->>'canopy_account_id', observed_at DESC
		) latest
	`).Scan(&resp.TrackedSocialAccounts, &resp.TotalFollowers)
	if err != nil {
		fail()
		return
	}

	resp.ClientsByStatus, err = h.queryStatusBreakdown(ctx, "crm", "client.observed", "crm_client_id")
	if err != nil {
		fail()
		return
	}

	resp.SocialAccountsByStatus, err = h.queryStatusBreakdown(ctx, "crm", "account.observed", "crm_social_account_id")
	if err != nil {
		fail()
		return
	}

	err = h.IntelligenceDB.QueryRow(ctx, `
		SELECT coalesce(sum(new_subs), 0) FROM (
			SELECT DISTINCT ON (payload->>'crm_client_id', payload->>'metric_date')
				coalesce((payload->>'new_subs_gained')::bigint, 0) AS new_subs
			FROM source_events
			WHERE producer = 'crm' AND event_type = 'subscriber.metrics'
				AND (payload->>'metric_date')::date >= current_date - interval '7 days'
			ORDER BY payload->>'crm_client_id', payload->>'metric_date', observed_at DESC
		) latest
	`).Scan(&resp.NewSubscribers7d)
	if err != nil {
		fail()
		return
	}

	rows, err := h.IntelligenceDB.Query(ctx, `
		SELECT DISTINCT ON (producer)
			producer,
			coalesce(payload->>'collector', ''),
			observed_at,
			coalesce((payload->'counts'->>'accepted')::bigint, 0),
			coalesce((payload->'counts'->>'rejected')::bigint, 0),
			coalesce((payload->'counts'->>'quarantined')::bigint, 0),
			(payload->>'errors') IS NOT NULL
		FROM source_events
		WHERE event_type = 'collection.run'
		ORDER BY producer, observed_at DESC
	`)
	if err != nil {
		fail()
		return
	}
	defer rows.Close()
	for rows.Next() {
		var c intelligenceCollectorHealth
		var observedAt time.Time
		if err := rows.Scan(&c.Producer, &c.Collector, &observedAt, &c.Accepted, &c.Rejected, &c.Quarantined, &c.HasErrors); err != nil {
			fail()
			return
		}
		c.ObservedAt = observedAt.UTC().Format(time.RFC3339)
		resp.CollectionHealth = append(resp.CollectionHealth, c)
	}
	if rows.Err() != nil {
		fail()
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// queryStatusBreakdown counts the latest row per entity id (keyed by
// idField, e.g. "crm_client_id") grouped by its current payload->>'status'.
// A ledger row is a point-in-time observation, not a current-state table —
// this collapses re-observations down to "what do we currently believe" the
// same way the business-summary queries above do.
func (h *Handler) queryStatusBreakdown(ctx context.Context, producer, eventType, idField string) ([]intelligenceStatusBreakdown, error) {
	rows, err := h.IntelligenceDB.Query(ctx, `
		SELECT status, count(*) FROM (
			SELECT DISTINCT ON (payload->>$3)
				coalesce(payload->>'status', 'unknown') AS status
			FROM source_events
			WHERE producer = $1 AND event_type = $2
			ORDER BY payload->>$3, observed_at DESC
		) latest
		GROUP BY status
		ORDER BY count(*) DESC
	`, producer, eventType, idField)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []intelligenceStatusBreakdown
	for rows.Next() {
		var s intelligenceStatusBreakdown
		if err := rows.Scan(&s.Status, &s.Count); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
