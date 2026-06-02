package registry

import (
	"github.com/google/go-containerregistry/pkg/authn"
)

// Keychain returns an authenticator chain for registry API calls (digest resolution).
func (c Credentials) Keychain() (authn.Keychain, error) {
	var chains []authn.Keychain

	if c.Username != "" || c.Password != "" {
		chains = append(chains, staticBasicKeychain{
			username: c.Username,
			password: c.Password,
		})
	}

	if c.DockerConfigDir != "" {
		chains = append(chains, dockerConfigDirKeychain{dir: c.DockerConfigDir})
	}

	if len(chains) == 0 {
		return authn.DefaultKeychain, nil
	}
	return authn.NewMultiKeychain(chains...), nil
}

type staticBasicKeychain struct {
	username string
	password string
}

func (s staticBasicKeychain) Resolve(authn.Resource) (authn.Authenticator, error) {
	if s.username == "" && s.password == "" {
		return authn.Anonymous, nil
	}
	return &authn.Basic{Username: s.username, Password: s.password}, nil
}
