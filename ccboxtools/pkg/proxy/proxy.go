// Package proxy translates the proxy's live allow file — its rendered tinyproxy ERE
// filter rules — into the bare domains the proxy allows.
package proxy

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"strings"
)

// The filter rule's shape around the escaped domain: the prefix admits the apex and
// any subdomain; the anchor ends the rule.
const (
	erePrefix = `(^|\.)`
	ereSuffix = `$`
)

// ereLine matches one rendered filter rule, capturing its escaped domain — Render's
// shape, quoted.
var ereLine = regexp.MustCompile(`^` + regexp.QuoteMeta(erePrefix) + `(.+)` + regexp.QuoteMeta(ereSuffix) + `$`)

// Render turns each bare domain into the allow file's ERE filter rule ((^|\.)domain$),
// matching the apex and any subdomain.
func Render(domains []string) []byte {
	var b strings.Builder
	for _, d := range domains {
		b.WriteString(erePrefix + regexp.QuoteMeta(d) + ereSuffix + "\n")
	}
	return []byte(b.String())
}

// Domains reads the allow file at path into its bare domains, one per filter rule.
// A missing file means the run has no proxy (no_proxy).
func Domains(path string) ([]string, error) {
	body, err := os.ReadFile(path) // #nosec G304 -- the command's fixed mount
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("proxy: no allow file at %s — this run has no proxy (no_proxy)", path)
	}
	if err != nil {
		return nil, err
	}
	return Parse(string(body))
}

// Parse translates the allow file's ERE filter rules into their bare domains, in file
// order. A malformed rule errors: the file is the proxy's own render, never hand-written.
func Parse(body string) ([]string, error) {
	var domains []string
	scanner := bufio.NewScanner(strings.NewReader(body))
	for lineNo := 1; scanner.Scan(); lineNo++ {
		if strings.TrimSpace(scanner.Text()) == "" {
			continue
		}
		domain, err := unrender(scanner.Text())
		if err != nil {
			return nil, fmt.Errorf("proxy: line %d: %w", lineNo, err)
		}
		domains = append(domains, domain)
	}
	return domains, scanner.Err()
}

// unrender strips one filter rule's (^|\.) prefix and $ suffix, then unescapes its
// domain's dots.
func unrender(line string) (string, error) {
	m := ereLine.FindStringSubmatch(line)
	if m == nil {
		return "", fmt.Errorf("malformed filter rule %q", line)
	}
	domain := strings.ReplaceAll(m[1], `\.`, ".")
	if strings.Contains(domain, `\`) {
		return "", fmt.Errorf("malformed escape in filter rule %q", line)
	}
	return domain, nil
}
