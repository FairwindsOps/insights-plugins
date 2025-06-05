package ci

func (ci *CIScan) TerraformEnabled() bool {
	return *ci.config.Reports.TFSec.Enabled
}
