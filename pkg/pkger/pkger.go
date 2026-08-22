// Package pkger resolves coding-CLI versions from their release channels,
// pinning the moving ref ("latest") into the concrete version an image build records.
package pkger

// Pinner resolves a CLI release channel's "latest" into a concrete version.
type Pinner interface {
	Latest() (string, error)
}

// Pkger describes a CLI's install source: pinning plus the scheme the image
// build's PKGER arg carries, rendered by Arg.
type Pkger interface {
	Pinner
	Arg() string
}
