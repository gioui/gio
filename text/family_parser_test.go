package text

import (
	"testing"

	"golang.org/x/exp/slices"
)

func TestParser(t *testing.T) {
	type scenario struct {
		variantName string
		input       string
	}
	type testcase struct {
		name      string
		inputs    []scenario
		expected  []string
		shouldErr bool
	}

	for _, tc := range []testcase{
		{
			name: "empty",
			inputs: []scenario{
				{
					variantName: "",
				},
			},
			shouldErr: true,
		},
		{
			name: "comma failure",
			inputs: []scenario{
				{
					variantName: "bare single",
					input:       ",",
				},
				{
					variantName: "bare multiple",
					input:       ",, ,,",
				},
			},
			shouldErr: true,
		},
		{
			name: "comma success",
			inputs: []scenario{
				{
					variantName: "squote",
					input:       "','",
				},
				{
					variantName: "dquote",
					input:       `","`,
				},
			},
			expected: []string{","},
		},
		{
			name: "comma success multiple",
			inputs: []scenario{
				{
					variantName: "squote",
					input:       "',,', ',,'",
				},
				{
					variantName: "dquote",
					input:       `",,", ",,"`,
				},
			},
			expected: []string{",,", ",,"},
		},
		{
			name: "backslashes",
			inputs: []scenario{
				{
					variantName: "bare",
					input:       `\font\\`,
				},
				{
					variantName: "dquote",
					input:       `"\\font\\\\"`,
				},
				{
					variantName: "squote",
					input:       `'\\font\\\\'`,
				},
			},
			expected: []string{`\font\\`},
		},
		{
			name: "invalid backslashes",
			inputs: []scenario{
				{
					variantName: "dquote",
					input:       `"\\""`,
				},
				{
					variantName: "squote",
					input:       `'\\''`,
				},
			},
			shouldErr: true,
		},
		{
			name: "too many quotes",
			inputs: []scenario{
				{
					variantName: "dquote",
					input:       `"""`,
				},
				{
					variantName: "squote",
					input:       `'''`,
				},
			},
			shouldErr: true,
		},
		{
			name: "serif serif's serif\"s",
			inputs: []scenario{
				{
					variantName: "bare",
					input:       `serif, serif's, serif"s`,
				},
				{
					variantName: "squote",
					input:       `'serif', 'serif\'s', 'serif"s'`,
				},
				{
					variantName: "dquote",
					input:       `"serif", "serif's", "serif\"s"`,
				},
			},
			expected: []string{"serif", `serif's`, `serif"s`},
		},
		{
			name: "complex list",
			inputs: []scenario{
				{
					variantName: "bare",
					input:       `Times New Roman, Georgia Common, Helvetica Neue, serif`,
				},
				{
					variantName: "squote",
					input:       `'Times New Roman', 'Georgia Common', 'Helvetica Neue', 'serif'`,
				},
				{
					variantName: "dquote",
					input:       `"Times New Roman", "Georgia Common", "Helvetica Neue", "serif"`,
				},
				{
					variantName: "mixed",
					input:       `Times New Roman, "Georgia Common", 'Helvetica Neue', "serif"`,
				},
				{
					variantName: "mixed with weird spacing",
					input:       `Times New Roman  ,"Georgia Common"              , 'Helvetica Neue' ,"serif"`,
				},
			},
			expected: []string{"Times New Roman", "Georgia Common", "Helvetica Neue", "serif"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var p parser
			for _, scen := range tc.inputs {
				t.Run(scen.variantName, func(t *testing.T) {
					actual, err := p.parse(scen.input)
					if (err != nil) != tc.shouldErr {
						t.Errorf("unexpected error state: %v", err)
					}
					if !slices.Equal(tc.expected, actual) {
						t.Errorf("expected\n%q\ngot\n%q", tc.expected, actual)
					}
				})
			}
		})
	}
}
