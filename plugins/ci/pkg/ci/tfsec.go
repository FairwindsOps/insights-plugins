package ci

const DefaultCustomCheckRuleID = "tfsec_custom_check"

func (ci *CIScan) TerraformEnabled() bool {
	return *ci.config.Reports.TFSec.Enabled
}
