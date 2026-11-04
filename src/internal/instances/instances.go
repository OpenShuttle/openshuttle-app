// Package instances lets an instance of the app find the one holding a
// tournament file. While it holds a file, an instance listens on a local
// socket named after that file; another instance (or a process launched by a
// double-click on the file) connects to it and asks it to come to the front.
//
// The sockets are AF_UNIX sockets (Windows 10 has them too): local only, no
// network permission prompt. Nothing in them is state: a socket file left by
// a crash answers no one and is replaced by the next holder.
package instances

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	raiseRequest = "raise"
	raiseReply   = "ok"
	// timeout bounds a whole exchange: a holder that does not answer in time
	// is treated as absent.
	timeout = 2 * time.Second
)

// Rendezvous is the folder holding the sockets, one per held file.
type Rendezvous struct {
	Dir string
}

// Default is the per-user rendezvous: the runtime folder on Linux
// ($XDG_RUNTIME_DIR), else the user's temporary folder (per user on Windows
// and macOS), in an "openshuttle" subfolder only its owner can enter.
func Default() Rendezvous {
	base := os.Getenv("XDG_RUNTIME_DIR")
	if base == "" {
		base = os.TempDir()
	}
	name := "openshuttle"
	if uid := os.Getuid(); uid >= 0 {
		// /tmp is shared on Linux: one folder per user.
		name += "-" + strconv.Itoa(uid)
	}
	return Rendezvous{Dir: filepath.Join(base, name)}
}

// Endpoint is the socket of the instance holding path. Every spelling of one
// file gives one endpoint: the path is made absolute, symbolic links are
// resolved and, where file names ignore case, it is folded.
func (r Rendezvous) Endpoint(path string) string {
	sum := sha256.Sum256([]byte(Key(path)))
	// Short: a socket path is limited to about 100 bytes.
	return filepath.Join(r.Dir, hex.EncodeToString(sum[:8])+".sock")
}

// Key is the canonical spelling of a file path.
func Key(path string) string {
	p, err := filepath.Abs(path)
	if err != nil {
		p = filepath.Clean(path)
	}
	if real, err := filepath.EvalSymlinks(p); err == nil {
		p = real
	}
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		p = strings.ToLower(p)
	}
	return p
}

// Hold answers for path until the returned stop is called: each request runs
// raise. The caller holds the file's lock, so no other live instance listens
// on this endpoint; a socket file left by a crash is replaced.
func (r Rendezvous) Hold(path string, raise func()) (stop func(), err error) {
	if err := os.MkdirAll(r.Dir, 0o700); err != nil {
		return nil, err
	}
	ep := r.Endpoint(path)
	l, err := net.Listen("unix", ep)
	if err != nil {
		// Left by a crash: nobody answers there.
		if conn, dialErr := net.DialTimeout("unix", ep, timeout); dialErr == nil {
			conn.Close()
			return nil, err
		}
		if rmErr := os.Remove(ep); rmErr != nil && !errors.Is(rmErr, os.ErrNotExist) {
			return nil, err
		}
		if l, err = net.Listen("unix", ep); err != nil {
			return nil, err
		}
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				serve(conn, raise)
			}()
		}
	}()
	var once sync.Once
	return func() {
		once.Do(func() {
			l.Close() // removes the socket file
			wg.Wait()
		})
	}, nil
}

func serve(conn net.Conn, raise func()) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(timeout))
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil || strings.TrimSpace(line) != raiseRequest {
		return
	}
	raise()
	_, _ = conn.Write([]byte(raiseReply + "\n"))
}

// Raise asks the instance holding path to come to the front. False when no
// instance answers for it.
func (r Rendezvous) Raise(path string) bool {
	conn, err := net.DialTimeout("unix", r.Endpoint(path), timeout)
	if err != nil {
		return false
	}
	defer conn.Close()
	// Windows only lets the foreground process hand the foreground over.
	allowForeground()
	_ = conn.SetDeadline(time.Now().Add(timeout))
	if _, err := conn.Write([]byte(raiseRequest + "\n")); err != nil {
		return false
	}
	line, err := bufio.NewReader(conn).ReadString('\n')
	return err == nil && strings.TrimSpace(line) == raiseReply
}
