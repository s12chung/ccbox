package docker

// ProxyOptions configures the egress wall on a run
type ProxyOptions struct {
	HostDir string // the live host dir, bound read-only at pkginfo.ProxyMount

	BeforeStart func() error // before the proxy starts
	OnRefresh   func() error // on refreshing proxy
	OnStop      func() error // runs once the proxy is torn down

	LogFile string // the host file the proxy's own log lines append to
}
