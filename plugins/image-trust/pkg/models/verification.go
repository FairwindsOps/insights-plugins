package models

// VerificationMode identifies the verification strategy used for an image.
type VerificationMode string

const (
	VerificationModeCosignKeyless VerificationMode = "cosign-keyless"
)

// VerificationObservation is the raw result returned by a verifier.
type VerificationObservation struct {
	Mode    VerificationMode
	Status  Status
	Reason  string
	Signer  SignerDetails
	Signers []SignerDetails
}
