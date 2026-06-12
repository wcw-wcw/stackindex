package models

type Severity string

const (
	SeverityHigh   Severity = "high"
	SeverityMedium Severity = "medium"
	SeverityLow    Severity = "low"
	SeverityInfo   Severity = "info"
)

type Finding struct {
	Severity       Severity `json:"severity"`
	Category       string   `json:"category"`
	Message        string   `json:"message"`
	File           string   `json:"file,omitempty"`
	Recommendation string   `json:"recommendation,omitempty"`
}
