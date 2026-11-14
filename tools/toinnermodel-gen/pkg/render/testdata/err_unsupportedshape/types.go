// Package errunsupportedshape is toinnermodel-gen's fixture for the unsupported-shape refusal: a
// multi-level pointer and a T -> *T field are generation errors, never guesses.
package errunsupportedshape

//toinnermodel:gen
type safe struct {
	Kept  *string
	Deep  **string
	Plain string
}

type Safe struct {
	Kept  *string
	Deep  *string
	Plain *string
}
