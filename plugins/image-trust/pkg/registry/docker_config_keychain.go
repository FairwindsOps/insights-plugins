package registry

import (
	"encoding/base64"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
)

type dockerConfigDirKeychain struct {
	dir string
}

func (k dockerConfigDirKeychain) Resolve(target authn.Resource) (authn.Authenticator, error) {
	cfg, err := readDockerConfigDir(k.dir)
	if err != nil {
		return authn.Anonymous, err
	}

	registry := strings.ToLower(target.RegistryStr())
	candidates := registryCandidates(registry)
	for _, candidate := range candidates {
		if auth, ok := cfg.Auths[candidate]; ok {
			if authenticator := authConfigAuthenticator(auth); authenticator != nil {
				return authenticator, nil
			}
		}
	}
	return authn.Anonymous, nil
}

func authConfigAuthenticator(auth dockerConfigAuth) authn.Authenticator {
	if auth.Identity != "" {
		return &authn.Bearer{Token: auth.Identity}
	}

	username := auth.Username
	password := auth.Password
	if username == "" && password == "" && auth.Auth != "" {
		decoded, err := base64.StdEncoding.DecodeString(auth.Auth)
		if err != nil {
			return nil
		}
		parts := strings.SplitN(string(decoded), ":", 2)
		if len(parts) == 2 {
			username = parts[0]
			password = parts[1]
		}
	}
	if username == "" && password == "" {
		return nil
	}
	return &authn.Basic{Username: username, Password: password}
}

func registryCandidates(registry string) []string {
	registry = strings.TrimSpace(strings.ToLower(registry))
	if registry == "" {
		return nil
	}

	candidates := []string{registry}
	if !strings.Contains(registry, "://") {
		candidates = append(candidates, "https://"+registry, "http://"+registry)
	}
	for _, candidate := range append([]string(nil), candidates...) {
		candidates = append(candidates, candidate+"/", candidate+"/v1/", candidate+"/v1")
	}
	if registry == "index.docker.io" || strings.HasSuffix(registry, ".docker.io") {
		candidates = append(candidates, "https://index.docker.io/v1/", "index.docker.io")
	}
	return candidates
}
