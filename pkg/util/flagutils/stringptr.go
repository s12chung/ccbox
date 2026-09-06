package flagutils

// StringPtr returns a flag value writing through to p: Set allocates, so an
// unset flag leaves *p nil — String/StringVar can only offer a "" default.
func StringPtr(p **string) StringPtrValue { return StringPtrValue{p} }

// StringPtrValue is StringPtr's value; Type carries it beyond flag.Value to pflag.Value.
type StringPtrValue struct{ p **string }

func (v StringPtrValue) String() string {
	if v.p == nil || *v.p == nil { // v.p is nil on flag's zero-value reflection
		return ""
	}
	return **v.p
}

// Set stores a pointer to s into the target.
func (v StringPtrValue) Set(s string) error {
	*v.p = &s
	return nil
}

// Type returns the flag's value type name.
func (v StringPtrValue) Type() string { return "string" }
