package sandbox

// Compile-time check that DefaultBackend satisfies Backend.
var _ Backend = DefaultBackend{}
var _ Backend = (*DefaultBackend)(nil)
