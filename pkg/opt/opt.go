// Package opt provides utilities for handling optional parameters with sensible defaults.
//
// It enables a pattern where functions accept variadic parameters to simulate optional arguments,
// allowing callers to omit parameters and use defaults, or provide custom values.
//
// Pattern:
// Traditional Go doesn't support optional parameters. This package provides a convenient way to
// emulate them using variadic arguments combined with a default value.
//
// Usage:
//
// Instead of:
//
//	func Foo(param string) { ... }      // Forced to always provide param
//	func FooWithDefault(param ...string) { ... }  // Caller must know about defaults
//
// Use:
//
//	func Foo(param ...string) {
//		p := opt.Opt("default-value", param)
//		// p is "default-value" if param is empty, otherwise param[0]
//	}
//
// Callers can now use:
//
//	Foo()              // Uses default "default-value"
//	Foo("custom")      // Uses "custom"
//
// Example:
//
//	func WithTimeout(timeout ...time.Duration) ConfigOption {
//		return func(cfg *Config) {
//			cfg.Timeout = opt.Opt(30*time.Second, timeout)
//		}
//	}
//
//
// Note:
// Only the first element of the variadic parameter is used. Additional elements are ignored.
// This is intentional to simplify the API and prevent confusion.
package opt

// Opt returns opts[0] if possible, otherwise def
func Opt[S ~[]T, T any](def T, opts S) T {
	if len(opts) > 0 {
		return opts[0]
	}
	return def
}
