package yamlkit

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/s12chung/firm"
	"github.com/s12chung/firm/rule"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/util/osutil"
)

func TestValue(t *testing.T) {
	foo := "foo"
	tests := []struct {
		name     string
		v        any
		comments []string
		want     string
		wantErr  string
	}{
		{"nil interface", nil, nil, "", ""},
		{"nil pointer", (*string)(nil), nil, "", ""},
		{"nil slice", []string(nil), nil, "", ""},
		{"nil map", map[string]string(nil), nil, "", ""},
		{"pointer value", &foo, nil, " foo", ""},
		{"string", "claude", nil, " claude", ""},
		{"bool", true, nil, " true", ""},
		{"quoted string", "yes", nil, " \"yes\"", ""}, // yaml quotes bool-like strings

		{"empty slice", []string{}, nil, " []", ""},
		{"empty map", map[string]string{}, nil, " {}", ""},
		{"slice", []string{"a", "b"}, nil, "\n  - a\n  - b", ""},
		{"map sorts keys", map[string]string{"b": "1", "a": "2"}, nil, "\n  a: \"2\"\n  b: \"1\"", ""},

		{"comment beside plain value", "claude", []string{"the cli"}, " claude # the cli", ""},
		{"comment beside pointer value", &foo, []string{"why"}, " foo # why", ""},
		{"empty comment beside plain value", "claude", []string{""}, " claude", ""},
		{"comment beside unset", nil, []string{"why"}, " # why", ""},
		{"comment beside empty slice", []string{}, []string{"why"}, "", "comments: got 1, want 0 or none"},
		{"too many comments beside plain value", "claude", []string{"a", "b"}, "", "comments: got 2, want none or one"},
		{"slice comments", []string{"a", "b"}, []string{"first", "second"}, "\n  - a # first\n  - b # second", ""},
		{"slice empty comment", []string{"a", "b"}, []string{"", "second"}, "\n  - a\n  - b # second", ""},
		{"slice comment count mismatch", []string{"a", "b"}, []string{"only"}, "", "comments: got 1, want 2 or none"},
		{"map comments", map[string]string{"b": "1", "a": "2"}, []string{"a's", "b's"}, "\n  a: \"2\" # a's\n  b: \"1\" # b's", ""},
		{"pointer map comments", &map[string]string{"a": "1"}, []string{"why"}, "\n  a: \"1\" # why", ""},
		{
			"struct entry comments",
			[]entry{{"a", "1"}, {"b", "2"}},
			[]string{"first", "second"},
			"\n  - name: a # first\n    val: \"1\"\n  - name: b # second\n    val: \"2\"", "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Value(tt.v, tt.comments)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFlow(t *testing.T) {
	tests := []struct {
		name   string
		values []string
		groups map[string]int
		want   string
	}{
		{"nil values render bare", nil, nil, ""},
		{"empty values render explicit emptiness", []string{}, nil, " []"},
		{
			"entries sharing a row index share a line",
			[]string{".ccbox.yaml", "secrets"},
			map[string]int{".ccbox.yaml": 0, "secrets": 0},
			" [\n  .ccbox.yaml, secrets,\n]",
		},
		{
			"lines come out in row order, not first-seen",
			[]string{"**/*.pem", ".env"},
			map[string]int{"**/*.pem": 1, ".env": 0},
			" [\n  .env,\n  '**/*.pem',\n]",
		},
		{
			"entries outside groups get a line each, after the numbered",
			[]string{".ccbox.yaml", ".typo", "secrets", ".env"},
			map[string]int{".ccbox.yaml": 1, "secrets": 1, ".env": 2},
			" [\n  .ccbox.yaml, secrets,\n  .env,\n  .typo,\n]",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Flow(tt.values, tt.groups))
		})
	}
}

type decoded struct{ Name string }

func init() {
	firm.MustRegisterType(firm.NewDefinition[decoded]().
		Validates(firm.RuleMap{
			"Name": {rule.Match{Regexp: regexp.MustCompile(`^[a-z]+$`)}},
		}))
}

func TestValidatedDecode(t *testing.T) {
	preset := decoded{Name: "preset"}
	tests := []struct {
		name       string
		body       string
		setDefault func(decoded) decoded
		want       decoded
		wantErr    string
	}{
		{"decodes and validates", "name: web\n", nil, decoded{Name: "web"}, ""},
		{
			"setDefault runs before validation",
			"name: BAD\n",
			func(d decoded) decoded {
				d.Name = "web"
				return d
			},
			decoded{Name: "web"},
			"",
		},
		{"unknown field errors", "name: web\nnope: 1\n", nil, preset, "field nope not found"},
		{"invalid value errors", "name: BAD\n", nil, preset, "does not match"},
		{"invalid yaml errors", "name: [unclosed\n", nil, preset, "did not find expected"},
		{"empty body decodes to the zero value", "", func(d decoded) decoded { d.Name = "web"; return d }, decoded{Name: "web"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidatedDecode[decoded]([]byte(tt.body), tt.setDefault)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestReadLayers(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		path := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(path, []byte(body), osutil.File))
		return path
	}
	present := write("present.yaml", "name: web\n")
	empty := write("empty.yaml", "")
	commentOnly := write("comments.yaml", "# nothing\n")
	missing := filepath.Join(dir, "missing.yaml")

	layers, err := ReadLayers[decoded]([]string{missing, present, empty, commentOnly})
	require.NoError(t, err)
	assert.Equal(t, []decoded{{}, {Name: "web"}, {}, {}}, layers)

	for name, body := range map[string]string{
		"unknown field": "name: web\nnope: 1\n",
		"invalid yaml":  "name: [unclosed\n",
	} {
		t.Run(name+" errors", func(t *testing.T) {
			bad := write("bad.yaml", body)
			_, err := ReadLayers[decoded]([]string{bad})
			require.ErrorContains(t, err, "parse "+bad+":")
		})
	}
}

func TestMergeLayers(t *testing.T) {
	merge := func(dst *decoded, src decoded) {
		if src.Name != "" {
			dst.Name = src.Name
		}
	}

	got := MergeLayers([]decoded{{Name: "web"}, {}, {Name: "db"}}, merge)
	assert.Equal(t, decoded{Name: "db"}, got, "folds layers onto a zero value, the later layer winning")
}

type entry struct {
	Name string
	Val  string
}

// TestValue_MultilineEntries pins a large multi-entry block — struct fields, nested maps
// and sequences — with its comments beside the entry lines, as a readable literal
func TestValue_MultilineEntries(t *testing.T) {
	type server struct {
		Name  string
		Image string
		Env   map[string]string
		Cmds  []string
	}

	v := []server{
		{Name: "web", Image: "nginx", Env: map[string]string{"PORT": "8080", "USER": "app"}, Cmds: []string{"migrate", "serve"}},
		{Name: "db", Image: "postgres", Env: map[string]string{"PGDATA": "/var/lib/postgres"}, Cmds: []string{"initdb", "start"}},
	}
	got, err := Value(v, []string{"serves the app", "the database"})
	require.NoError(t, err)
	assert.Equal(t, `
  - name: web # serves the app
    image: nginx
    env:
      PORT: "8080"
      USER: app
    cmds:
      - migrate
      - serve
  - name: db # the database
    image: postgres
    env:
      PGDATA: /var/lib/postgres
    cmds:
      - initdb
      - start`, got)
}

func TestIndent(t *testing.T) {
	assert.Equal(t, "\n    - a\n    - b", Indent(2, "\n  - a\n  - b"), "prefixes each line, nesting a Value block under an indented key")
	assert.Equal(t, "\n  - a\n\n  - b", Indent(2, "\n- a\n\n- b"), "empty lines stay bare")
	assert.Equal(t, "  plain", Indent(2, "plain"))
	assert.Equal(t, "no change", Indent(0, "no change"))
}

func TestStripProse(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			"prose and trailing comments drop, comment-only lines stay",
			"## prose\na: 1 ## why\n# - dist\n",
			"a: 1\n# - dist\n",
		},
		{
			"an example's prose tail drops, its code stays",
			"#   tcp: \"5432\" ## PostgreSQL\n",
			"#   tcp: \"5432\"\n",
		},
		{"indented prose drops", "a:\n  ## prose\n  b: 1\n", "a:\n  b: 1\n"},
		{"a trailing hash pair is a trailing comment", "a: 1 ## why\n", "a: 1\n"},
		{"a quoted single hash stays", "a: \"x # y\"\n", "a: \"x # y\"\n"},
		{"an escaped quote holds to the real comment", "a: \"x\\\"y\" ## why\n", "a: \"x\\\"y\"\n"},
		{"a hash glued to a token stays", "a: b#c\n", "a: b#c\n"},
		{"a glued hash pair stays", "a: b##c\n", "a: b##c\n"},
		{"a line ending in a hash stays", "a: #\n", "a: #\n"},
		{
			"blank runs prose leaves behind collapse, the ends trimmed",
			"## head\n\n\na: 1\n\n\n## mid\n\nb: 2\n\n\n",
			"a: 1\n\nb: 2\n",
		},
		{"only prose strips to nothing", "## all\n## prose\n", ""},
		{"a bare hash pair is prose", "##\na: 1\n", "a: 1\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, StripProse(tt.body))
		})
	}
}

func TestCommentFirstEntry(t *testing.T) {
	assert.Nil(t, CommentFirstEntry(nil, "why"))
	assert.Equal(t, []string{"why", "", ""}, CommentFirstEntry([]string{"a", "b", "c"}, "why"))
}

func TestEntryToComments_Slice(t *testing.T) {
	assert.Nil(t, EntryToComments([]string(nil), nil))
	m := map[string]string{"a": "a's", "c": "c's"}
	assert.Equal(t, []string{"a's", "", "c's"}, EntryToComments([]string{"a", "b", "c"}, m))
}

func TestEntryToComments_Map(t *testing.T) {
	assert.Nil(t, EntryToComments(map[string]string(nil), nil))
	m := map[string]string{"gitconfig": "special value"}
	// yaml sorts its keys its own way — symbols before letters
	assert.Equal(t, []string{"", "special value"}, EntryToComments(map[string]string{"gitconfig": "enabled", "~/.aws": "~/.aws"}, m))
}
