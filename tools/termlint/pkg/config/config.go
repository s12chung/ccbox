// Package config loads and validates the .termlint.json schema.
package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/s12chung/ccbox/tools/termlint/pkg/pattern"
)

// Config is the .termlint.json schema. Terms ban a word's generated forms
// (pattern.Compile); allows are phrases whose match makes overlapping term
// hits on the same line allowed.
type Config struct {
	Extensions   []string `json:"extensions"`
	ExcludeGlobs []string `json:"exclude_globs"`
	Terms        []Term   `json:"terms"`
}

// Term bans word in any casing and separator style, with an optional hint
// printed alongside its violations.
type Term struct {
	Word   string   `json:"word"`
	Allows []string `json:"allows"`
	Hint   string   `json:"hint"`
}

// Load reads and validates the term config at path.
func Load(path string) (*Config, error) {
	content, err := os.ReadFile(path) // #nosec G304 -- the linter's own config
	if err != nil {
		return nil, err
	}
	var config Config
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &config, config.validate()
}

func (c *Config) validate() error {
	if len(c.Extensions) == 0 {
		return errors.New("extensions: at least one required")
	}
	for _, term := range c.Terms {
		if term.Word == "" {
			return errors.New("term: word is required")
		}
		for _, allow := range term.Allows {
			if len(pattern.Words(allow)) == 0 {
				return fmt.Errorf("term %q: allow %q has no words", term.Word, allow)
			}
		}
	}
	return nil
}
