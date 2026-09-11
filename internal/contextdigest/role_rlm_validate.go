package contextdigest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/behaviorengineering/majordomo/internal/config"
	jmodules "github.com/behaviorengineering/majordomo/internal/judge/modules"
	stropdspy "github.com/behaviorengineering/strop/dspy"
	"github.com/behaviorengineering/strop/dspy/factory"
	typroles "github.com/behaviorengineering/typology/roles"
)

const (
	agreementMatch      = "match"
	agreementDisagree   = "disagree"
	agreementAbstain    = "abstain"
	agreementUnvalidated = "unvalidated"

	confidenceAgreeMatch   = 0.95
	confidenceConflictBar  = 0.40
	rlmValidateWorkers     = 4
	maxRLMValidatePackages = 64
)

var roleTokenRE = regexp.MustCompile(`(?i)\b(role|final)\s*[:=]\s*([a-z_]+)`)

// packageRoleRLMValidator classifies one package from AST context via RLM.
type packageRoleRLMValidator interface {
	Validate(ctx context.Context, pkgPath, contextMD, mechanicalRole, mechanicalEvidence string) (role, evidence string, iterations int, err error)
}

type stropPackageRoleRLM struct {
	module interface {
		Complete(ctx context.Context, contextPayload any, query string) (response string, iterations int, err error)
	}
}

type rlmCompleteAdapter struct {
	complete func(ctx context.Context, contextPayload any, query string) (string, int, error)
}

func (a rlmCompleteAdapter) Complete(ctx context.Context, contextPayload any, query string) (string, int, error) {
	return a.complete(ctx, contextPayload, query)
}

func newStropPackageRoleRLM(ctx context.Context, cfg config.RepoConfig) (packageRoleRLMValidator, error) {
	provider, ok, err := cfg.ResolveTaskProvider(jmodules.TaskTypologyInspect)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("typology_inspect provider not configured")
	}
	stropProvider := provider.ToStrop()
	llmFactory := factory.NewLLMFactory(nil, 3*time.Minute)
	llm, err := llmFactory.CreateLLM(ctx, stropProvider)
	if err != nil {
		return nil, fmt.Errorf("typology_inspect RLM LLM: %w", err)
	}
	rlmCfg := stropdspy.RLMDefaults()
	rlmCfg.MaxFullContextQueryChars = 24_000
	rlmCfg.Timeout = provider.GetTimeout(3 * time.Minute)
	module, err := stropdspy.CreateRLMModule(llm, rlmCfg)
	if err != nil {
		return nil, err
	}
	return stropPackageRoleRLM{
		module: rlmCompleteAdapter{complete: func(ctx context.Context, contextPayload any, query string) (string, int, error) {
			answer, result, err := stropdspy.RLMComplete(ctx, module, contextPayload, query)
			if err != nil {
				return "", 0, err
			}
			iters := 0
			if result != nil {
				iters = result.Iterations
			}
			return answer, iters, nil
		}},
	}, nil
}

func (v stropPackageRoleRLM) Validate(ctx context.Context, pkgPath, contextMD, mechanicalRole, mechanicalEvidence string) (string, string, int, error) {
	query := fmt.Sprintf(`Classify this Go package into exactly one role.
Allowed roles: entrypoint, server, dto, exec_runner, aggregator, adapter, config, observability, unknown.
Mechanical prior: role=%s evidence=%s
MUST NOT use the directory basename as evidence.
Explore exports and bodies in the context; dig into private helpers only if needed.
End with lines:
role: <one allowed role>
evidence: <short symbol quotes>
Package path (not evidence): %s`, mechanicalRole, mechanicalEvidence, pkgPath)
	answer, iters, err := v.module.Complete(ctx, contextMD, query)
	if err != nil {
		return "", "", iters, err
	}
	role, evidence := parseRLMRoleAnswer(answer)
	return role, evidence, iters, nil
}

func parseRLMRoleAnswer(text string) (role, evidence string) {
	lower := strings.ToLower(text)
	for _, line := range strings.Split(text, "\n") {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(trim), "evidence:") {
			evidence = strings.TrimSpace(trim[len("evidence:"):])
		}
	}
	if m := roleTokenRE.FindStringSubmatch(text); len(m) == 3 {
		role = normalizeObservedRole(m[2])
	}
	if role == "" {
		for _, cand := range []string{
			roleEntrypoint, roleHTTPSurface, roleDTO, roleExecRunner, roleAggregator,
			roleAdapter, roleConfig, roleObservability, roleUnknown,
		} {
			if strings.Contains(lower, "role: "+cand) || strings.Contains(lower, "role="+cand) {
				role = cand
				break
			}
		}
	}
	if role == "" {
		role = roleUnknown
	}
	return role, evidence
}

func normalizeObservedRole(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case roleEntrypoint, roleHTTPSurface, roleDTO, roleExecRunner, roleAggregator,
		roleAdapter, roleConfig, roleObservability, roleUnknown:
		return strings.ToLower(strings.TrimSpace(s))
	case "http_surface":
		return roleHTTPSurface
	default:
		return ""
	}
}

// validatePackageRolesRLM runs RLM validation for every package and rewrites confidence.
func validatePackageRolesRLM(ctx context.Context, validator packageRoleRLMValidator, analysisDir, evidenceDir, rolesPath, rolesYAML string) (string, error) {
	if validator == nil {
		return rolesYAML, nil
	}
	doc := mustParseRoles(rolesYAML)
	if len(doc.Packages) == 0 {
		return rolesYAML, nil
	}
	rlmContextPath := filepath.Join(evidenceDir, "package_rlm_context.md")
	wholeContext, _ := os.ReadFile(rlmContextPath)

	pkgs := doc.Packages
	if len(pkgs) > maxRLMValidatePackages {
		pkgs = pkgs[:maxRLMValidatePackages]
	}

	type result struct {
		idx        int
		node       packageRoleNode
		err        error
		loggedRole string
	}
	results := make([]result, len(pkgs))
	sem := make(chan struct{}, rlmValidateWorkers)
	var wg sync.WaitGroup
	for i, n := range pkgs {
		wg.Add(1)
		go func(i int, n packageRoleNode) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			ctxMD := packageRLMContextSnippet(string(wholeContext), n.Path)
			if strings.TrimSpace(ctxMD) == "" {
				built, err := typroles.FormatPackageRLMContextForPath(analysisDir, n.Path, nil)
				if err == nil {
					ctxMD = built
				}
			}
			if strings.TrimSpace(ctxMD) == "" {
				results[i] = result{idx: i, node: markUnvalidated(n), err: fmt.Errorf("empty RLM context")}
				return
			}
			mechRole := n.Role
			if mechRole == "" {
				mechRole = roleUnknown
			}
			llmRole, evidence, iters, err := validator.Validate(ctx, n.Path, ctxMD, mechRole, strings.Join(n.Evidence, ", "))
			if err != nil {
				node := markUnvalidated(n)
				node.Evidence = appendUnique(node.Evidence, "rlm_error:"+truncateErr(err))
				results[i] = result{idx: i, node: node, err: err, loggedRole: ""}
				return
			}
			if rejectInspectRoleContradiction(llmRole, ctxMD) {
				llmRole = roleUnknown
				evidence = strings.TrimSpace(evidence + " contradiction_rejected")
			}
			updated := applyRLMAgreement(n, llmRole, evidence, iters)
			results[i] = result{idx: i, node: updated, loggedRole: llmRole}
		}(i, n)
	}
	wg.Wait()

	byPath := roleByPath(doc)
	for _, r := range results {
		if r.err != nil {
			logf("WARN", "typology RLM validate %s: %v", doc.Packages[r.idx].Path, r.err)
		}
		byPath[normalizeRolePath(r.node.Path)] = r.node
	}
	outPkgs := make([]packageRoleNode, 0, len(doc.Packages))
	for _, n := range doc.Packages {
		if updated, ok := byPath[normalizeRolePath(n.Path)]; ok {
			outPkgs = append(outPkgs, updated)
			continue
		}
		outPkgs = append(outPkgs, n)
	}
	doc.Packages = outPkgs
	if err := writePackageRoles(rolesPath, doc); err != nil {
		return "", err
	}
	data, err := os.ReadFile(rolesPath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func markUnvalidated(n packageRoleNode) packageRoleNode {
	n.MechanicalRole = firstNonEmpty(n.MechanicalRole, n.Role)
	n.Agreement = agreementUnvalidated
	return n
}

func applyRLMAgreement(n packageRoleNode, llmRole, evidence string, iterations int) packageRoleNode {
	mech := n.Role
	if mech == "" {
		mech = roleUnknown
	}
	n.MechanicalRole = mech
	n.LLMRole = llmRole
	n.RLMIterations = iterations
	if evidence != "" {
		n.Evidence = appendUnique(n.Evidence, "rlm:"+evidence)
	} else {
		n.Evidence = appendUnique(n.Evidence, "rlm_validate")
	}

	stage1 := isStage1DeliveryRole(mech, n)

	switch {
	case llmRole == roleUnknown:
		n.Agreement = agreementAbstain
		n.Role = mech
		n.InspectedStage = 3
	case llmRole == mech && mech != roleUnknown:
		n.Agreement = agreementMatch
		n.Role = mech
		n.Confidence = confidenceAgreeMatch
		n.CandidateRole = ""
		n.InspectedStage = 3
	case stage1 && llmRole != mech:
		n.Agreement = agreementDisagree
		n.Role = mech
		n.Evidence = appendUnique(n.Evidence, "rlm_disagree_kept_stage1")
		n.InspectedStage = 3
	case mech == roleUnknown && llmRole != roleUnknown:
		n.Agreement = agreementMatch
		n.Role = llmRole
		n.Confidence = llmInspectConfidence
		n.CandidateRole = ""
		n.InspectedStage = 3
	default:
		n.Agreement = agreementDisagree
		n.Role = roleUnknown
		n.Confidence = confidenceConflictBar
		n.CandidateRole = firstNonEmpty(llmRole, mech)
		n.Evidence = appendUnique(n.Evidence, "rlm_disagree_conflict")
		n.InspectedStage = 3
	}
	return n
}

func isStage1DeliveryRole(role string, n packageRoleNode) bool {
	if n.Confidence >= 0.90 {
		switch role {
		case roleEntrypoint, roleHTTPSurface, roleDTO, roleObservability:
			return true
		}
	}
	for _, e := range n.Evidence {
		switch e {
		case "has_main", "delivery:http", "delivery:ui", "delivery:grpc", "json_tags", "imports_otel", "imports_prometheus", "go_embed", "embeds_static":
			return true
		}
	}
	return false
}

func packageRLMContextSnippet(whole, pkgPath string) string {
	want := "## ./" + normalizeRolePath(pkgPath)
	alt := "## " + normalizeRolePath(pkgPath)
	idx := strings.Index(whole, want)
	if idx < 0 {
		idx = strings.Index(whole, alt)
	}
	if idx < 0 {
		return ""
	}
	rest := whole[idx:]
	next := strings.Index(rest[3:], "\n## ")
	if next >= 0 {
		return strings.TrimSpace(rest[:next+3])
	}
	return strings.TrimSpace(rest)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func truncateErr(err error) string {
	s := err.Error()
	if len(s) > 120 {
		return s[:120]
	}
	return s
}
