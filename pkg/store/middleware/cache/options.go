package cache

import "github.com/ing-bank/golibs/pkg/store"

var SupportedOptions = []store.Option{
	WithSkipCache,
	SkipCache,
}

var WithSkipCache, MatchSkipCache = store.SerializableBoolOptionBuilder("skipCache")

var SkipCache = WithSkipCache(true)
