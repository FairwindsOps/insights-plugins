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
	return modeEnabled(c.VerificationModes, ModeCosignKeyless)
}

func (c *Config) shouldEnableAttestationKey() bool {
	return modeEnabled(c.VerificationModes, ModeCosignKey)
}
