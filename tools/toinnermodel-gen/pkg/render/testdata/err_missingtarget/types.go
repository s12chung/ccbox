// Package errmissingtarget is toinnermodel-gen's fixture for the missing-target refusal: an
// annotated struct whose capitalized innerModel is absent is a generation error.
package errmissingtarget

//toinnermodel:gen
type widget struct {
	Name string
}
