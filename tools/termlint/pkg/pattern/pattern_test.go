package pattern

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompile_IsHit(t *testing.T) {
	tests := []struct {
		name   string
		word   string
		line   string
		want   bool
		allows []string
	}{
		{name: "Plain", word: "wall", line: "the wall holds", want: true},
		{name: "Camel", word: "wall", line: "x := NewWall()", want: true},
		{name: "AllowedOverlap", word: "wall", line: "the egress wall holds", allows: []string{"egress wall"}},
		{name: "AllowedAndBare", word: "wall", line: "the egress wall holds, unlike a bare wall", want: true, allows: []string{"egress wall"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, Compile(test.word, test.allows).IsHit(test.line), test.line)
		})
	}
}

func TestForms(t *testing.T) {
	tests := []struct {
		word string
		want []string
	}{
		{word: "wall", want: []string{"walling", "walled", "walls", "wall"}},
		{word: "ride", want: []string{"riding", "rides", "rided", "ride"}},
	}
	for _, test := range tests {
		assert.Equal(t, test.want, forms(test.word), test.word)
	}
}

func TestTermPattern_Matches(t *testing.T) {
	tests := []struct {
		name string
		line string
		want bool
	}{
		{name: "Plain", line: "the wall holds", want: true},
		{name: "Capitalized", line: "The Wall holds", want: true},
		{name: "Shouting", line: "THE WALL HOLDS", want: true},
		{name: "Plural", line: "two walls hold", want: true},
		{name: "PastTense", line: "it walled off", want: true},
		{name: "SnakeCase", line: "my_wall_var", want: true},
		{name: "CamelCase", line: "x := NewWall()", want: true},
		{name: "CamelPlural", line: "the egressWalls list", want: true},
		{name: "PunctuationBounds", line: "a wall, a proxy", want: true},
		{name: "SubstringNoRightBound", line: "swallow it", want: false},
		{name: "SubstringNoRightBoundMethod", line: "SwallowErr(err)", want: false},
		{name: "PartialWord", line: "the wallet is empty", want: false},
		{name: "PluralWithoutBound", line: "walledIn", want: false},
		{name: "JammedProse", line: "egresswall", want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, termPattern("wall").MatchString(test.line), test.line)
		})
	}
}

func TestTermPattern_Ride(t *testing.T) {
	tests := []struct {
		name string
		line string
		want bool
	}{
		{name: "Plain", line: "ride along", want: true},
		{name: "Plural", line: "free rides", want: true},
		{name: "Participle", line: "it is riding the queue", want: true},
		{name: "Capitalized", line: "Riding the queue", want: true},
		{name: "StemNotAForm", line: "get rid of it", want: false},
		{name: "AgentNoun", line: "a rider on the queue", want: false},
		{name: "Striding", line: "striding ahead", want: false},
		{name: "Deride", line: "they deride it", want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, termPattern("ride").MatchString(test.line), test.line)
		})
	}
}

func TestAllowPattern_Matches(t *testing.T) {
	tests := []struct {
		name   string
		phrase string
		line   string
		want   bool
	}{
		{name: "Spaced", phrase: "egress wall", line: "behind the egress wall", want: true},
		{name: "Hyphenated", phrase: "egress wall", line: "behind the egress-wall", want: true},
		{name: "Cased", phrase: "egress wall", line: "an EgressWall type", want: true},
		{name: "Underscored", phrase: "egress wall", line: "the egress_wall var", want: true},
		{name: "Jammed", phrase: "egress wall", line: "the egresswall var", want: true},
		{name: "PluralOverlaps", phrase: "egress wall", line: "the egress walls", want: true},
		{name: "OtherWord", phrase: "egress wall", line: "the firewall", want: false},
		{name: "Slashed", phrase: "wall/walled", line: `the "wall/walled" spelling`, want: true},
		{name: "SpacedSlashWords", phrase: "wall/walled", line: "wall walled pair", want: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, allowPattern(test.phrase).MatchString(test.line), test.line)
		})
	}
}

func TestWords(t *testing.T) {
	tests := []struct {
		phrase string
		want   []string
	}{
		{phrase: "egress wall", want: []string{"egress", "wall"}},
		{phrase: "wall/walled", want: []string{"wall", "walled"}},
		{phrase: "  spaced  out  ", want: []string{"spaced", "out"}},
		{phrase: " / ", want: []string{}},
	}
	for _, test := range tests {
		assert.Equal(t, test.want, Words(test.phrase), test.phrase)
	}
}
