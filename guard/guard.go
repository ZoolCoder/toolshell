// Package guard keeps a local admin page local. The page has no login: it
// listens on loopback only, answers only to a loopback Host (DNS rebinding),
// refuses writes another site started (CSRF), caps request bodies, and runs
// on an http.Server with timeouts that a slow client cannot hold open.
package guard

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/zoolcoder/toolshell"
)

// MaxBody is the default request body cap: forms on these pages are a few
// fields.
const MaxBody = 16 << 10

// LoopbackAddr refuses a listen address that is not loopback, so the page,
// and the credentials behind it, never reach the network.
func LoopbackAddr(addr string) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("address %q: %w", addr, err)
	}
	if host == "localhost" {
		return nil
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return nil
	}
	return fmt.Errorf("address %q is not loopback; the page has no login and listens on this machine only", addr)
}

// Options tunes Handler.
type Options struct {
	// Host is the address the server listens on; a request naming it is
	// always accepted. Loopback literals and localhost are accepted too.
	Host string
	// MaxBody caps a request body; 0 means MaxBody.
	MaxBody int64
	// Headers sets the response headers; nil means toolshell.SetHeaders.
	Headers func(http.Header)
}

// Handler wraps next with the Host check, the cross-site check on writes,
// the security headers, and the body cap. A POST form is parsed here, so a
// body over the cap is a 413 rather than a form read as empty.
func Handler(next http.Handler, opts Options) http.Handler {
	limit := opts.MaxBody
	if limit == 0 {
		limit = MaxBody
	}
	headers := opts.Headers
	if headers == nil {
		headers = toolshell.SetHeaders
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !HostAllowed(r.Host, opts.Host) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead && CrossSite(r) {
			http.Error(w, "cross-origin request refused", http.StatusForbidden)
			return
		}
		headers(w.Header())
		r.Body = http.MaxBytesReader(w, r.Body, limit)
		if r.Method == http.MethodPost && !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/") {
			if err := r.ParseForm(); err != nil {
				http.Error(w, "request too large", http.StatusRequestEntityTooLarge)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// CrossSite reports whether another site started the request. Browsers send
// Sec-Fetch-Site on every request, and it wins when present: under a strict
// referrer policy a same-origin form still sends Origin: null. same-site is
// refused too, since another port on this machine is the same site. Without
// it, an Origin naming a foreign host is enough to refuse.
func CrossSite(r *http.Request) bool {
	if site := r.Header.Get("Sec-Fetch-Site"); site != "" {
		return site != "same-origin" && site != "none"
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}
	u, err := url.Parse(origin)
	return err != nil || u.Host != r.Host
}

// HostAllowed reports whether a browser addressed this page by a name that
// cannot be an attacker's. DNS rebinding lets an attacker-controlled name
// resolve to 127.0.0.1, and in that attack the browser still sends the
// foreign Host it was told to use, so the Host must be the address the server
// was started on, or a loopback literal, which no public DNS name can be. It
// is not meant to stop a local process that forges Host; that process
// already has the port.
func HostAllowed(host, expected string) bool {
	if expected != "" && host == expected {
		return true
	}
	name := host
	if h, _, err := net.SplitHostPort(host); err == nil {
		name = h
	}
	name = strings.Trim(name, "[]")
	if strings.EqualFold(name, "localhost") {
		return true
	}
	ip := net.ParseIP(name)
	return ip != nil && ip.IsLoopback()
}

// Server is an http.Server with timeouts for a local page. WriteTimeout is
// long because a page may wait on remote APIs; pass a shorter one if not.
func Server(h http.Handler, writeTimeout time.Duration) *http.Server {
	if writeTimeout == 0 {
		writeTimeout = 3 * time.Minute
	}
	return &http.Server{
		Handler:           h,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       2 * time.Minute,
		MaxHeaderBytes:    64 << 10,
		// "OPTIONS *" is otherwise answered by net/http before the handler,
		// skipping the Host guard.
		DisableGeneralOptionsHandler: true,
	}
}

// Serve serves srv on l until ctx ends, then shuts down with a 5s grace.
func Serve(ctx context.Context, srv *http.Server, l net.Listener) error {
	errs := make(chan error, 1)
	go func() { errs <- srv.Serve(l) }()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	case err := <-errs:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

// ListenAndServe checks addr is loopback, listens, reports the bound address
// (the port may have been 0) to ready, then wraps h with Handler and serves
// until ctx ends.
func ListenAndServe(ctx context.Context, addr string, h http.Handler, ready func(host string)) error {
	if err := LoopbackAddr(addr); err != nil {
		return err
	}
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", addr, err)
	}
	defer func() { _ = l.Close() }()
	host := l.Addr().String()
	if ready != nil {
		ready(host)
	}
	return Serve(ctx, Server(Handler(h, Options{Host: host}), 0), l)
}
