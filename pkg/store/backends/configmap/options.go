package configmap

import (
	"github.com/ing-bank/golibs/pkg/store"
	labelstore "github.com/ing-bank/golibs/pkg/store/backends/labels"
)

// SupportedOptions is a list of options that are supported by the ConfigMap store.
// This is used for documentation purposes.
var SupportedOptions = []store.Option{
	labelstore.WithLabelSelector,
	labelstore.WithLabels,
	store.WithDryRun,   // Only for mutating calls
	store.WithPrefix,   // Only for list calls, not for Create/Update/Delete (for that use the prefix middleware)
	store.ListKeysOnly, // Only for list calls, leaves values empty
}
