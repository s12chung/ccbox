// Package errinputonly is toinnermodel-gen's fixture for the input-only-field refusal: an
// input field missing on its target is a generation error, never a dropped field.
package errinputonly

//toinnermodel:gen
type safe struct {
	Name  string
	Extra string // input-only: Safe has no Extra
}

type Safe struct {
	Name string
}
