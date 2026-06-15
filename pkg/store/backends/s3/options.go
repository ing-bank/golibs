package s3

import "github.com/ing-bank/golibs/pkg/store"

// SupportedOptions is a list of options that are supported by the Memory store.
// This is used for documentation purposes.
var SupportedOptions = []store.Option{
	store.WithDryRun,   // Only for mutating calls
	store.WithPrefix,   // Only for list calls, not for Create/Update/Delete (for that use the prefix middleware)
	store.ListKeysOnly, // Only for list calls, leaves values empty
}
