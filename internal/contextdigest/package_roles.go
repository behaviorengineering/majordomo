package contextdigest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	roleEntrypoint    = "entrypoint"
	roleHTTPSurface   = "server"
	roleDTO           = "dto"
	roleExecRunner    = "exec_runner"
	roleAggregator    = "aggregator"
	roleAdapter       = "adapter"
	roleConfig        = "config"
	roleObservability = "observability"
	roleUnknown       = "unknown"

	llmInspectConfidence = 0.75
)

func loadPackageRoles(path string) (packageRolesDoc, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return packageRolesDoc{}, err
	}
	var doc packageRolesDoc
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return packageRolesDoc{}, fmt.Errorf("parse package roles: %w", err)
	}
	return doc, nil
}

func writePackageRoles(path string, doc packageRolesDoc) error {
	data, err := yaml.Marshal(doc)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func roleByPath(doc packageRolesDoc) map[string]packageRoleNode {
	out := make(map[string]packageRoleNode, len(doc.Packages))
	for _, n := range doc.Packages {
		out[normalizeRolePath(n.Path)] = n
	}
	return out
}

func normalizeRolePath(p string) string {
	return filepath.ToSlash(strings.TrimPrefix(strings.TrimSpace(p), "./"))
}

func isInteractionRole(role string) bool {
	switch strings.TrimSpace(role) {
	case roleEntrypoint, roleHTTPSurface:
		return true
	default:
		return false
	}
}

func isExecRunnerRole(role string) bool {
	return strings.TrimSpace(role) == roleExecRunner
}

// rejectInspectRoleContradiction returns true when an LLM role contradicts mechanical source facts.
//
// RLM context markdown often carries Typology flags (importsOsExec: true) instead of
// quoted import paths. Treat those flags as evidence so agreementMatch can store.
func rejectInspectRoleContradiction(role, source string) bool {
	role = strings.TrimSpace(role)
	src := source
	hasOsExec := strings.Contains(src, `"os/exec"`) ||
		strings.Contains(src, "importsOsExec: true") ||
		strings.Contains(src, "imports_os_exec")
	hasHTTP := strings.Contains(src, `"net/http"`) || strings.Contains(src, "ServeHTTP") ||
		strings.Contains(src, "importsNetHTTP: true") || strings.Contains(src, "delivery:http")
	hasGRPC := strings.Contains(src, "google.golang.org/grpc") ||
		(strings.Contains(src, "Register") && strings.Contains(src, "Server")) ||
		strings.Contains(src, "importsGrpc: true") || strings.Contains(src, "delivery:grpc")
	hasEmbed := strings.Contains(src, "go:embed") ||
		strings.Contains(src, "goEmbed: true") || strings.Contains(src, "embedsStatic: true") ||
		strings.Contains(src, "embeds_static")
	hasOTel := strings.Contains(src, "go.opentelemetry.io/") ||
		strings.Contains(src, "importsOtel: true") || strings.Contains(src, "imports_otel")
	hasProm := strings.Contains(src, "github.com/prometheus/client_golang") ||
		strings.Contains(src, "importsPrometheus: true") || strings.Contains(src, "imports_prometheus")
	hasExportedFunc := strings.Contains(src, "\nfunc ") || strings.Contains(src, "\nfunc\t")
	switch role {
	case roleDTO:
		// Pure DTOs have no funcs; exported methods also appear as "func (".
		if hasExportedFunc || strings.Contains(src, "\nfunc (") {
			return true
		}
	case roleExecRunner:
		if !hasOsExec {
			return true
		}
	case roleHTTPSurface:
		if !hasHTTP && !hasGRPC && !hasEmbed {
			return true
		}
	case roleConfig:
		if hasOTel || hasProm {
			return true
		}
	case roleObservability:
		if !hasOTel && !hasProm {
			return true
		}
	}
	return false
}
