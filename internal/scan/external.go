package scan

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// External is an optional scanner run as a subprocess over a directory holding one folder per skill.
type External struct {
	Name string
	// Run returns findings; a scanner that ran and found problems is not an error.
	Run func(dir string) ([]Finding, error)
}

var Externals = []External{
	{Name: "gitleaks", Run: runGitleaks},
	{Name: "skill-scanner", Run: runSkillScanner},
}

// RunExternal runs each scanner found on PATH and names the ones that are missing.
func RunExternal(dir string) (findings []Finding, missing []string, err error) {
	for _, e := range Externals {
		if _, lerr := exec.LookPath(e.Name); lerr != nil {
			missing = append(missing, e.Name)
			continue
		}
		fs, rerr := e.Run(dir)
		if rerr != nil {
			return nil, nil, fmt.Errorf("%s: %w", e.Name, rerr)
		}
		findings = append(findings, fs...)
	}
	return findings, missing, nil
}

func runGitleaks(dir string) ([]Finding, error) {
	report, err := os.CreateTemp("", "gitleaks-*.json")
	if err != nil {
		return nil, err
	}
	report.Close()
	defer os.Remove(report.Name())
	cmd := exec.Command("gitleaks", "dir", dir, "--report-format", "json", "--report-path", report.Name(), "--exit-code", "0", "--no-banner")
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	data, err := os.ReadFile(report.Name())
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, nil
	}
	var leaks []struct {
		RuleID      string
		Description string
		File        string
		StartLine   int
	}
	if err := json.Unmarshal(data, &leaks); err != nil {
		return nil, fmt.Errorf("parse report: %w", err)
	}
	var out []Finding
	for _, l := range leaks {
		rel, rerr := filepath.Rel(dir, l.File)
		if rerr != nil {
			rel = l.File
		}
		skill, path, _ := strings.Cut(filepath.ToSlash(rel), "/")
		out = append(out, Finding{RuleID: "gitleaks:" + l.RuleID, Severity: High, Skill: skill, Path: path, Line: l.StartLine, Msg: l.Description})
	}
	return out, nil
}

// skill-scanner exits nonzero when a finding reaches --fail-on-severity.
func runSkillScanner(dir string) ([]Finding, error) {
	cmd := exec.Command("skill-scanner", "scan-all", dir, "--fail-on-severity", "high")
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil, nil
	}
	var ee *exec.ExitError
	if !errors.As(err, &ee) {
		return nil, err
	}
	return []Finding{{RuleID: "skill-scanner", Severity: High, Skill: "*", Msg: strings.TrimSpace(string(out))}}, nil
}
