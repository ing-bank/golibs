package logger

import "github.com/ing-bank/golibs/pkg/store"

var SupportedOptions = []store.Option{
	WithSkipLog,
	SkipLog,
}

var WithSkipLog, MatchSkipLog = store.SerializableBoolOptionBuilder("skipLog")

var SkipLog = WithSkipLog(true)
