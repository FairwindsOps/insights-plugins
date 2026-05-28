package models

// VerificationMode identifies the verification strategy used for an image.
type VerificationMode string

const (
	VerificationModeCosignKeyless VerificationMode = "cosign-keyless"
	VerificationModeCosignKey     VerificationMode = "cosign-key"
)

// VerificationObservation is the raw result returned by a verifier.
type VerificationObservation struct {
	Mode       VerificationMode
	VerifiedBy VerificationMode
	Status     Status
	Reason     string
	Signer     SignerDetails
	Signers    []SignerDetails
}
