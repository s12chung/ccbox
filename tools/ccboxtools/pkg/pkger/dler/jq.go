package dler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	yaml "github.com/itchyny/go-yaml"
	"github.com/s12chung/firm"
	"github.com/s12chung/firm/rule"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/httputil"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/jqutil"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/verify"
)

// errNoDownloadSource covers both download source modes missing.
var errNoDownloadSource = errors.New("dler: no download_template or jq_schema download_url")

// JQ handles the download from JQSchema
type JQ struct {
	pkginfo.JQSchema

	url string
	// unmarshal parses the url's document per the schema's format
	unmarshal func([]byte, any) error
	// client fetches through instead of the default client — nil; the test TLS
	// server injects its own
	client *http.Client
}

// NewJQ returns a new JQ
func NewJQ(url string, schema *pkginfo.JQSchema) (JQ, error) {
	if schema == nil || schema.DownloadURL == "" { // firm-validated; Go-constructed configs hit it
		return JQ{}, errNoDownloadSource
	}
	unmarshal, err := unmarshaler(schema.Format)
	if err != nil {
		return JQ{}, err
	}
	return JQ{url: url, JQSchema: *schema, unmarshal: unmarshal}, nil
}

// Latest resolves the version selector's result on the url's document
func (d JQ) Latest() (string, error) {
	doc, err := d.doc()
	if err != nil {
		return "", err
	}
	v, err := jqutil.SelectStr(d.Version, doc)
	if err != nil {
		return "", err
	}
	return guardVersion(d.url, v)
}

// doc fetches the url's document, parsed per the schema's format.
func (d JQ) doc() (any, error) {
	body, err := httputil.Body(d.client, d.url)
	if err != nil {
		return nil, err
	}
	var doc any
	if err := d.unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("parse %s: %w", d.Format, err)
	}
	return doc, nil
}

func unmarshaler(format string) (func([]byte, any) error, error) {
	switch format {
	case "yaml":
		return yaml.Unmarshal, nil
	case "json":
		return json.Unmarshal, nil
	default:
		return nil, fmt.Errorf("pkger: unknown format %q", format)
	}
}

// jqSchemaSelect carries the JQSchema's selectors for one document read;
// selectFields swaps each selector for its selected value.
type jqSchemaSelect struct {
	Version     string
	DownloadURL string
	Sha512      string
}

func jqSchemaSelectVldr(version string) firm.FieldsVldr[jqSchemaSelect] {
	return firm.Fields[jqSchemaSelect](firm.RuleMap{
		"Version":     {rule.Equal[string]{To: version}},
		"DownloadURL": {httpsURL},
		"Sha512":      {base64Sha512{}},
	})
}

// Download fetches the document once, guards it against rolling since Latest,
// then GETs the download_url selector's file through the sha512-verifying
// reader; the caller closes it.
func (d JQ) Download(version string) (io.ReadCloser, error) {
	doc, err := d.doc()
	if err != nil {
		return nil, err
	}

	sel, err := jqutil.SelectFields(jqSchemaSelect{
		Version:     d.Version,
		DownloadURL: d.DownloadURL,
		Sha512:      d.Sha512,
	}, doc)
	if err != nil {
		return nil, err
	}
	if err := jqSchemaSelectVldr(version).Validate(sel); err != nil {
		return nil, fmt.Errorf("%s: %w", d.url, err)
	}

	//nolint:bodyclose // the caller closes: the Sha512Reader wraps the body, the FilePkger.Install's defer drops it
	resp, err := httputil.GetOK(d.client, sel.DownloadURL)
	if err != nil {
		return nil, err
	}
	return verify.NewSha512Reader(resp.Body, sel.DownloadURL, sel.Sha512), nil
}
