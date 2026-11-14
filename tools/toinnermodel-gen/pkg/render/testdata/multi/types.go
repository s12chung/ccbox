// Package multi exercises the one package file: shared structTypes render once,
// in the first referencing pair's place, and a marked struct keeps its own.
package multi

//toinnermodel:gen
type alpha struct {
	Shared *shared
	Own    *own
}

type Alpha struct {
	Shared Shared
	Own    Own
}

//toinnermodel:gen
type beta struct {
	Shared *shared
}

type Beta struct {
	Shared Shared
}

//toinnermodel:gen
type gamma struct {
	Alpha *alpha
}

type Gamma struct {
	Alpha Alpha
}

// shared is referenced by alpha and beta, so it renders once, in alpha's place
type shared struct {
	Term string
}

type Shared struct {
	Term string
}

type own struct {
	Term string
}

type Own struct {
	Term string
}
