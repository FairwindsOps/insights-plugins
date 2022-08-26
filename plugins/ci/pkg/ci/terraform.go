package ci

import (
	"os/exec"
	"path/filepath"

	"github.com/fairwindsops/insights-plugins/plugins/ci/pkg/commands"
)

func (ci *CIScan) ProcessTerraformPaths() error {
	for _, terraformPath := range ci.config.Terraform.Paths {
		_, err := commands.ExecWithMessage(exec.Command("tfsec", filepath.Join(ci.repoBaseFolder, terraformPath)), "scanning Terraform in "+terraformPath)
		if err != nil {
			return err
		}
	}
	return nil
}
