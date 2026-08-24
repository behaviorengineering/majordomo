package report

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	findingRe     = regexp.MustCompile(`^\s*-\s+\[(CRITICAL|WARN|INFO)\]\s+(.+)$`)
	fileHeaderRe  = regexp.MustCompile(`^#\s+(.+)$`)
	metaRe        = regexp.MustCompile(`^\*\*(.+?):\*\*\s*(.+)$`)
	saDuplicateRe = regexp.MustCompile(`(?i)\(already flagged by static analysis\)`)
	xmlIllegalRe  = regexp.MustCompile("[\x00-\x08\x0b\x0c\x0e-\x1f\uFFFE\uFFFF]")
)

var skipFilenames = map[string]struct{}{
	"summary.md": {},
	"index.md":   {},
	"session.md": {},
}

const nameMax = 120

// Sanitize removes characters illegal in XML 1.0 element content.
func Sanitize(text string) string {
	// Strip illegal control chars; also drop unpaired surrogates by filtering runes.
	cleaned := xmlIllegalRe.ReplaceAllString(text, "")
	var b strings.Builder
	b.Grow(len(cleaned))
	for _, r := range cleaned {
		if r >= 0xD800 && r <= 0xDFFF {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

type finding struct {
	Level string
	Text  string
}

func parseReport(path string) (reviewedFile string, meta [][2]string, findings []finding, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", nil, nil, err
	}
	reviewedFile = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	for _, line := range strings.Split(string(data), "\n") {
		if m := fileHeaderRe.FindStringSubmatch(line); m != nil {
			reviewedFile = strings.TrimSpace(m[1])
			continue
		}
		if m := metaRe.FindStringSubmatch(line); m != nil {
			meta = append(meta, [2]string{strings.TrimSpace(m[1]), strings.TrimSpace(m[2])})
			continue
		}
		if m := findingRe.FindStringSubmatch(line); m != nil {
			findings = append(findings, finding{Level: m[1], Text: strings.TrimSpace(m[2])})
		}
	}
	return reviewedFile, meta, findings, nil
}

type xmlTestsuites struct {
	XMLName xml.Name     `xml:"testsuites"`
	Suites  []xmlSuite   `xml:"testsuite"`
}

type xmlSuite struct {
	Name      string        `xml:"name,attr"`
	Tests     string        `xml:"tests,attr"`
	Failures  string        `xml:"failures,attr"`
	Errors    string        `xml:"errors,attr"`
	Skipped   string        `xml:"skipped,attr"`
	Timestamp string        `xml:"timestamp,attr"`
	Cases     []xmlTestCase `xml:"testcase"`
}

type xmlTestCase struct {
	Classname string      `xml:"classname,attr"`
	Name      string      `xml:"name,attr"`
	Time      string      `xml:"time,attr"`
	SystemOut *xmlText    `xml:"system-out,omitempty"`
	Failure   *xmlFailure `xml:"failure,omitempty"`
	Skipped   *xmlSkipped `xml:"skipped,omitempty"`
}

type xmlText struct {
	Text string `xml:",chardata"`
}

type xmlFailure struct {
	Message string `xml:"message,attr"`
	Type    string `xml:"type,attr"`
	Text    string `xml:",chardata"`
}

type xmlSkipped struct {
	Message string `xml:"message,attr"`
	Text    string `xml:",chardata"`
}

// BuildTestsuite builds a JUnit suite from skill_dir/per-file/*.md (exported for tests).
func BuildTestsuite(skillDir, pipelineName, skillName string) (xmlSuite, error) {
	perFileDir := filepath.Join(skillDir, "per-file")
	var reportFiles []string
	if info, err := os.Stat(perFileDir); err == nil && info.IsDir() {
		entries, err := os.ReadDir(perFileDir)
		if err != nil {
			return xmlSuite{}, err
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			if _, skip := skipFilenames[e.Name()]; skip {
				continue
			}
			if strings.HasSuffix(e.Name(), "_session.md") {
				continue
			}
			reportFiles = append(reportFiles, filepath.Join(perFileDir, e.Name()))
		}
		sort.Strings(reportFiles)
	}

	suite := xmlSuite{
		Name:      "Copilot PR Review — " + pipelineName + "/" + skillName,
		Tests:     "0",
		Failures:  "0",
		Errors:    "0",
		Skipped:   "0",
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
	}
	total, failures, skipped := 0, 0, 0

	for _, reportPath := range reportFiles {
		reviewedFile, meta, findings, err := parseReport(reportPath)
		if err != nil {
			return xmlSuite{}, err
		}
		classname := "copilot_review." + pipelineName + "." + skillName + "." +
			strings.ReplaceAll(strings.ReplaceAll(reviewedFile, "\\", "/"), "/", ".")
		var metaLines []string
		for _, m := range meta {
			metaLines = append(metaLines, m[0]+": "+m[1])
		}
		metaBlock := strings.Join(metaLines, "\n")
		preamble := strings.TrimRight("File: "+reviewedFile+"\n"+metaBlock+"\n", "\n") + "\n"

		if len(findings) == 0 {
			suite.Cases = append(suite.Cases, xmlTestCase{
				Classname: classname,
				Name:      reviewedFile,
				Time:      "0",
				SystemOut: &xmlText{Text: Sanitize(preamble + "\nNo issues found.")},
			})
			total++
			continue
		}

		for _, f := range findings {
			if saDuplicateRe.MatchString(f.Text) {
				continue
			}
			name := f.Text
			if utf8.RuneCountInString(name) > nameMax {
				runes := []rune(name)
				name = string(runes[:nameMax-3]) + "..."
			}
			tc := xmlTestCase{
				Classname: classname,
				Name:      "[" + f.Level + "] " + name,
				Time:      "0",
				SystemOut: &xmlText{Text: Sanitize(preamble + "\n[" + f.Level + "] " + f.Text)},
			}
			body := Sanitize(preamble + "\n[" + f.Level + "] " + f.Text)
			if f.Level == "CRITICAL" {
				tc.Failure = &xmlFailure{
					Message: Sanitize(f.Text),
					Type:    f.Level,
					Text:    body,
				}
				failures++
			} else {
				tc.Skipped = &xmlSkipped{
					Message: Sanitize(f.Text),
					Text:    body,
				}
				skipped++
			}
			suite.Cases = append(suite.Cases, tc)
			total++
		}
	}

	suite.Tests = strconv.Itoa(total)
	suite.Failures = strconv.Itoa(failures)
	suite.Skipped = strconv.Itoa(skipped)
	return suite, nil
}

// ConvertToJUnit walks review output and writes one JUnit XML per pipeline/skill.
func ConvertToJUnit(reviewOutputDir, junitOutputDir string) error {
	info, err := os.Stat(reviewOutputDir)
	if err != nil || !info.IsDir() {
		return nil
	}
	if err := os.MkdirAll(junitOutputDir, 0o755); err != nil {
		return err
	}

	pipelineEntries, err := os.ReadDir(reviewOutputDir)
	if err != nil {
		return err
	}
	sort.Slice(pipelineEntries, func(i, j int) bool {
		return pipelineEntries[i].Name() < pipelineEntries[j].Name()
	})

	for _, pe := range pipelineEntries {
		if !pe.IsDir() {
			continue
		}
		pipelineName := pe.Name()
		pipelineDir := filepath.Join(reviewOutputDir, pipelineName)
		skillEntries, err := os.ReadDir(pipelineDir)
		if err != nil {
			return err
		}
		sort.Slice(skillEntries, func(i, j int) bool {
			return skillEntries[i].Name() < skillEntries[j].Name()
		})
		for _, se := range skillEntries {
			if !se.IsDir() {
				continue
			}
			skillName := se.Name()
			skillDir := filepath.Join(pipelineDir, skillName)
			suite, err := BuildTestsuite(skillDir, pipelineName, skillName)
			if err != nil {
				return err
			}
			doc := xmlTestsuites{Suites: []xmlSuite{suite}}
			data, err := xml.MarshalIndent(doc, "", "  ")
			if err != nil {
				return err
			}
			out := append([]byte(xml.Header), data...)
			out = append(out, '\n')
			outPath := filepath.Join(junitOutputDir, "copilot-review-"+pipelineName+"-"+skillName+".xml")
			if err := os.WriteFile(outPath, out, 0o644); err != nil {
				return err
			}
		}
	}
	return nil
}
