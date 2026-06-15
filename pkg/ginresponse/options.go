package ginresponse

type Option func(*Wrapper) error

func WithErrorToBody(f func(err error) any) Option {
	return func(w *Wrapper) error {
		w.ErrorToBody = f
		return nil
	}
}

func WithErrorToStatus(f func(err error) int) Option {
	return func(w *Wrapper) error {
		w.ErrorToStatus = f
		return nil
	}
}

// SkipResponseLog is an option to skip logging the response.
func SkipResponseLog(skip bool) Option {
	return func(w *Wrapper) error {
		w.SkipResponseLog = skip
		return nil
	}
}
