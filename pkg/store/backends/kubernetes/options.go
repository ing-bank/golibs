package kubernetes

import (
	"fmt"

	"github.com/ing-bank/golibs/pkg/config"
	"github.com/ing-bank/golibs/pkg/store"
	labelstore "github.com/ing-bank/golibs/pkg/store/backends/labels"
)

// Option is a configuration option for DynamicResource.
type Option[V GenericType] = config.Option[*DynamicResource[V]]

// WithLabelsEnricher sets a labels enricher function for dynamically adding labels to resources.
func WithLabelsEnricher[V GenericType](enricher LabelsEnricher[V]) config.Opt[*DynamicResource[V]] {
	return func(d *DynamicResource[V]) error {
		d.labelsEnricher = enricher
		return nil
	}
}

// SupportedOptions is a list of options that are supported by the Kubernetes store.
// This is used for documentation purposes.
var SupportedOptions = []store.Option{
	labelstore.WithLabelSelector,
	labelstore.WithLabels,
	store.WithDryRun,   // Only for mutating calls
	store.WithPrefix,   // Only for list calls, not for Create/Update/Delete (for that use the prefix middleware)
	store.ListKeysOnly, // Only for list calls, leaves values empty
	WithResolveConflict,
	WithSubResourceOnly,
}

// BuildLabels builds the complete set of labels for a resource by combining immutable labels and enriched labels.
func (c *DynamicResource[V]) BuildLabels(obj V, opts *[]store.Option) (map[string]string, error) {
	return labelstore.BuildLabels(obj, c.cfg.ImmutableLabels, c.labelsEnricher, opts)
}

// WithSubResourceOnly is an option to operate only on subresources (e.g., status).
// MatchSubResourceOnly retrieves the subresource-only value from options.
var WithSubResourceOnly, MatchSubResourceOnly = store.SerializableBoolOptionBuilder("subresource")

// WithResolveConflict is an option for enabling conflict resolution in apply operations.
// MatchResolveConflict retrieves the resolve conflict value from options.
// When used, operations should automatically resolve conflicts by using values from the modified configuration.
// This is useful for update operations where the client wants to override changes in the live configuration
// without manually handling conflicts.
var WithResolveConflict, MatchResolveConflict = store.SerializableBoolOptionBuilder("resolveConflict")

// CreateOption holds options for create operations.
type CreateOption struct {
	DryRun bool
}

// UpdateOption holds options for update operations.
type UpdateOption struct {
	DryRun          bool
	SubResourceOnly bool
}

// ApplyOption holds options for apply operations.
type ApplyOption struct {
	DryRun          bool
	ResolveConflict bool
	SubResourceOnly bool
}

// ListOption holds options for list operations.
type ListOption struct {
	LabelSelector string
	ListKeysOnly  bool
	Prefix        string
}

// DeleteOption holds options for delete operations.
type DeleteOption struct {
	DryRun bool
}

// buildCreateOptions extracts and validates create options.
// Returns an error if any unsupported options remain after extraction.
func buildCreateOptions(opts []store.Option) (CreateOption, error) {
	if len(opts) == 0 {
		return CreateOption{}, nil
	}
	dryRun, _ := store.MatchDryRun(&opts)
	o := CreateOption{
		DryRun: dryRun,
	}
	if err := store.CheckOptionsExhausted(opts); err != nil {
		return CreateOption{}, err
	}
	return o, nil
}

// buildUpdateOptions extracts and validates update options.
// Returns an error if any unsupported options remain after extraction.
func buildUpdateOptions(opts []store.Option) (UpdateOption, error) {
	if len(opts) == 0 {
		return UpdateOption{}, nil
	}
	dryRun, _ := store.MatchDryRun(&opts)
	subResourceOnly, _ := MatchSubResourceOnly(&opts)
	o := UpdateOption{
		DryRun:          dryRun,
		SubResourceOnly: subResourceOnly,
	}
	if err := store.CheckOptionsExhausted(opts); err != nil {
		return UpdateOption{}, err
	}
	return o, nil
}

// buildDeleteOptions extracts and validates delete options.
// Returns an error if any unsupported options remain after extraction.
func buildDeleteOptions(opts []store.Option) (DeleteOption, error) {
	if len(opts) == 0 {
		return DeleteOption{}, nil
	}
	dryRun, _ := store.MatchDryRun(&opts)
	o := DeleteOption{
		DryRun: dryRun,
	}
	if err := store.CheckOptionsExhausted(opts); err != nil {
		return DeleteOption{}, err
	}
	return o, nil
}

// buildApplyOptions extracts and validates apply options.
// Returns an error if any unsupported options remain after extraction.
func buildApplyOptions(opts []store.Option) (ApplyOption, error) {
	if len(opts) == 0 {
		return ApplyOption{}, nil
	}
	dryRun, _ := store.MatchDryRun(&opts)
	subresourceonly, _ := MatchSubResourceOnly(&opts)
	resolveConflict, _ := MatchResolveConflict(&opts)

	o := ApplyOption{
		DryRun:          dryRun,
		SubResourceOnly: subresourceonly,
		ResolveConflict: resolveConflict,
	}
	if err := store.CheckOptionsExhausted(opts); err != nil {
		return ApplyOption{}, err
	}
	return o, nil
}

// buildListOptions extracts and validates list options.
// Returns an error if any unsupported options remain after extraction.
func (c *DynamicResource[V]) buildListOptions(opts []store.Option) (ListOption, error) {
	if len(opts) == 0 {
		return ListOption{}, nil
	}
	prefix, _ := store.MatchPrefix(&opts)
	listKeyOnly, _ := store.MatchListKeyOnly(&opts)
	selector, _ := labelstore.MatchLabelSelector(&opts)
	labelSelector, err := labelstore.GenerateLabelSelector(c.cfg.ImmutableLabels, selector)
	if err != nil {
		return ListOption{}, fmt.Errorf("failed to parse label selector: %w", err)
	}

	o := ListOption{
		Prefix:        prefix,
		LabelSelector: labelSelector.String(),
		ListKeysOnly:  listKeyOnly,
	}
	if err := store.CheckOptionsExhausted(opts); err != nil {
		return ListOption{}, err
	}
	return o, nil
}
