package config

type Option[T any] func(T) error

func ApplyOptions[T any](base T, opts ...Option[T]) error {
	for _, o := range opts {
		if o == nil {
			continue
		}
		if err := o(base); err != nil {
			return err
		}
	}
	return nil
}
