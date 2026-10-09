// Package flagptr adapts flag variables for the stdlib flag and pflag.
package flagptr

// String returns a flag value writing through to p: Set allocates, so an
// unset flag leaves *p nil — flag.StringVar can only offer a "" default.
func String(p **string) Value { return Value{p} }

// Value is String's value; Type carries it beyond flag.Value to pflag.Value.
type Value struct{ p **string }

func (v Value) String() string {
	if v.p == nil || *v.p == nil { // v.p is nil on flag's zero-value reflection
		return ""
	}
	return **v.p
}

// Set stores a pointer to s into the target.
func (v Value) Set(s string) error {
	*v.p = &s
	return nil
}

// Type returns the flag's value type name.
func (v Value) Type() string { return "string" }
