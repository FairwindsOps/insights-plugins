package models

import "time"

type Status string

const (
	StatusVerified          Status = "verified"
	StatusUnsigned          Status = "unsigned"
	StatusSignedUntrusted   Status = "signed_untrusted"
	StatusVerificationError Status = "verification_error"
	StatusUnknown           Status = "unknown"
)

// SignerDetails captures signer information when available.
type SignerDetails struct {
	Issuer  string `json:"issuer,omitempty"`
	Subject string `json:"subject,omitempty"`
	KeyRef  string `json:"keyRef,omitempty"`
}

// ImageTrustResult is the final per-image trust state sent to Insights.
type ImageTrustResult struct {
	Name             string          `json:"name"`
	ID               string          `json:"id"`
	PullRef          string          `json:"pullRef"`
	Status           Status          `json:"status"`
	Reason           string          `json:"reason,omitempty"`
	VerificationMode string          `json:"verificationMode,omitempty"`
	VerifiedBy       string          `json:"verifiedBy,omitempty"`
	Allowlisted      bool            `json:"allowlisted"`
	AllowlistReason  string          `json:"allowlistReason,omitempty"`
	Owners           []Resource      `json:"owners"`
	Signer           SignerDetails   `json:"signer"`
	CandidateSigners []SignerDetails `json:"-"`
	LastCheckedAt    time.Time       `json:"lastCheckedAt"`
}

// Finding is a derived action item for non-compliant images.
type Finding struct {
	ResourceNamespace string  `json:"ResourceNamespace"`
	ResourceKind      string  `json:"ResourceKind"`
	ResourceName      string  `json:"ResourceName"`
	Title             string  `json:"Title"`
	Description       string  `json:"Description"`
	Remediation       string  `json:"Remediation"`
	Severity          float64 `json:"Severity"`
	Category          string  `json:"Category"`
}

// Summary aggregates image-trust statuses across all images in the report.
type Summary struct {
	TotalImages       int `json:"totalImages"`
	Verified          int `json:"verified"`
	Unsigned          int `json:"unsigned"`
	SignedUntrusted   int `json:"signedUntrusted"`
	VerificationError int `json:"verificationError"`
	Unknown           int `json:"unknown"`
	Allowlisted       int `json:"allowlisted"`
}

// Report is the top-level image-trust report payload.
type Report struct {
	Images   []ImageTrustResult `json:"images"`
	Summary  Summary            `json:"summary"`
	Findings []Finding          `json:"ActionItems,omitempty"`
}
