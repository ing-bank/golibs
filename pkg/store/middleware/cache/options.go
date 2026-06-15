package cache

import "github.com/ing-bank/golibs/pkg/store"

var SupportedOptions = []store.Option{}

type SkipCache bool

func (d SkipCache) Bool() bool {
	return bool(d)
}

func (d SkipCache) Serialize() (string, string) {
	if d {
		return "skipCache", "true"
	}
	return "skipCache", "false"
}

var WithSkipCache, MatchSkipCache = store.SerializableOptionBuilder[SkipCache]("skipCache", func(val string) (store.Option, error) {
	return SkipCache(val == "true"), nil
})
