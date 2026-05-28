package config

// ResolveVerificationModes appends attestation modes when attestations are enabled or types are configured.
func (c *Config) ResolveVerificationModes() {
	if !c.wantsAttestations() {
		return
	}

	modes := dedupeModes(append([]string(nil), c.VerificationModes...))

	if c.shouldEnableAttestationKeyless() && !modeEnabled(modes, ModeCosignAttestationKeyless) {
		modes = append(modes, ModeCosignAttestationKeyless)
	}
	if c.shouldEnableAttestationKey() && !modeEnabled(modes, ModeCosignAttestationKey) {
		modes = append(modes, ModeCosignAttestationKey)
	}

	c.VerificationModes = dedupeModes(modes)
}

func (c *Config) wantsAttestations() bool {
	return c.AttestationsEnabled || len(c.AttestationTypes) > 0
}

func (c *Config) shouldEnableAttestationKeyless() bool {
	return modeEnabled(c.VerificationModes, ModeCosignKeyless) || c.hasKeylessTrustPolicy()
}

func (c *Config) shouldEnableAttestationKey() bool {
	return modeEnabled(c.VerificationModes, ModeCosignKey) || c.hasPublicKeyConfig()
}

func (c *Config) hasKeylessTrustPolicy() bool {
	return len(c.TrustedIssuers) > 0 || len(c.TrustedSubjects) > 0 || len(c.TrustedSubjectREs) > 0
}

func (c *Config) hasPublicKeyConfig() bool {
	return len(c.PublicKeyPaths) > 0 || len(c.PublicKeyRefs) > 0 || c.PublicKeyDir != ""
}
