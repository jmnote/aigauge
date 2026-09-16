package providers

import "encoding/json"

// agyUsageResponse mirrors `agy -p /usage --output-format json`'s stdout
// field-for-field - this is the actual wire format AI Gauge reads for
// Antigravity, since agy's CLI is the only supported path to that quota data
// (see antigravity.go). ParseAntigravityUsage converts it into
// AntigravityUsage below; this type does no conversion of its own.
type agyUsageResponse struct {
	Command struct {
		Data struct {
			Description string `json:"description"`
			Groups      []struct {
				Name        string `json:"name"`
				Description string `json:"description"`
				Buckets     []struct {
					ID                string  `json:"id"`
					Name              string  `json:"name"`
					Description       string  `json:"description"`
					Window            string  `json:"window"`
					RemainingFraction float64 `json:"remaining_fraction"`
					ResetTime         string  `json:"reset_time"`
				} `json:"buckets"`
			} `json:"groups"`
		} `json:"data"`
	} `json:"command"`
}

// AntigravityUsage is agy's `/usage` response converted into AI Gauge's own
// shape. FetchedAt isn't part of that response; it's stamped on after the
// fact (the local clock, at fetch time). ToDisplay (antigravity.go) does all
// sorting/labeling; this type does none.
type AntigravityUsage struct {
	Groups      []AntigravityUsageGroup `json:"groups"`
	Description string                  `json:"description"`
	FetchedAt   string                  `json:"fetchedAt"`

	// Raw is the complete, untrimmed agy stdout ParseAntigravityUsage was
	// given, so nothing is silently dropped for a future ToDisplay to draw on.
	Raw json.RawMessage `json:"-"`

	DiagnosisFields
}

type AntigravityUsageGroup struct {
	DisplayName string                   `json:"displayName"`
	Description string                   `json:"description"`
	Buckets     []AntigravityUsageBucket `json:"buckets"`
}

type AntigravityUsageBucket struct {
	BucketID          string  `json:"bucketId"`
	DisplayName       string  `json:"displayName"`
	Window            string  `json:"window"`
	Description       string  `json:"description"`
	RemainingFraction float64 `json:"remainingFraction"`
	ResetTime         string  `json:"resetTime"`
}
