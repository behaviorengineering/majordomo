// Package githttps builds git -c http.extraHeader args for forge HTTPS remotes.
package githttps

import forgeauth "github.com/behaviorengineering/gitvalet/pkg/auth"

// ExtraHeaderArgs returns git -c http.extraHeader=... args for forge HTTPS auth.
func ExtraHeaderArgs(token, scm string) []string {
	return forgeauth.GitConfigArgs(forgeauth.Credential{Token: token, SCM: scm})
}

// InferSCM guesses the forge from an HTTPS remote URL host.
func InferSCM(remoteURL string) string {
	return forgeauth.InferSCM(remoteURL)
}
