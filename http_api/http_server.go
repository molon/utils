package http_api

import (
	"log"
	"net"
	"net/http"
	"strings"

	"github.com/molon/utils/lg"
)

type logWriter struct {
	logf lg.AppLogFunc
}

func (l logWriter) Write(p []byte) (int, error) {
	l.logf(lg.WARN, "%s", string(p))
	return len(p), nil
}

func Serve(listener net.Listener, handler http.Handler, logf lg.AppLogFunc) {
	logf(lg.INFO, "HTTP: listening on %s", listener.Addr())

	server := &http.Server{
		Handler:  handler,
		ErrorLog: log.New(logWriter{logf}, "", 0),
	}
	err := server.Serve(listener)
	// theres no direct way to detect this error because it is not exposed
	if err != nil && !strings.Contains(err.Error(), "use of closed network connection") {
		logf(lg.ERROR, "http.Serve() - %s", err)
	}

	logf(lg.INFO, "HTTP: closing %s", listener.Addr())
}
