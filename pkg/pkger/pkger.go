// Package pkger resolves coding-CLI versions from their release channels,
// pinning the moving ref ("latest") into the concrete version an image build records.
package pkger

// Pinner resolves a CLI release channel's "latest" into a concrete version.
type Pinner interface {
	Latest() (string, error)
}
