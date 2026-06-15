package utils

import (
	"os"
	"os/signal"
	"syscall"
)

// StopSignal returns a channel which will be closed if the SIGINT or SIGTERM signal is received
func StopSignal() <-chan struct{} {
	return WatchSignals(syscall.SIGINT, // Ctrl+C
		syscall.SIGTERM, // Termination Request
	)
}

// WatchSignals returns a channel which will be closed if a signal is received
func WatchSignals(sig ...os.Signal) <-chan struct{} {
	stop := make(chan struct{}, 1)
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, sig...)
		<-c
		close(stop)
	}()
	return stop
}

func SendKill(sig syscall.Signal) error {
	return syscall.Kill(os.Getpid(), sig)
}
