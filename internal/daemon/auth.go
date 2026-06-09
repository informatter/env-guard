package daemon

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

type peerCredKeyType struct{}

var peerCredKey peerCredKeyType

// PeerCred represents the credentials of a peer process connecting to the daemon.
// PID, UID, GID are obtained from SO_PEERCRED. StartTime is read from /proc/<pid>/stat
type PeerCred struct {
	PID       int
	UID       int
	GID       int
	StartTime uint64
}

func credFromContext(ctx context.Context) (*PeerCred, bool) {
	c, ok := ctx.Value(peerCredKey).(*PeerCred)
	return c, ok
}

// extractCred extracts the peer credentials from a Unix domain socket connection using SO_PEERCRED.
// Returns nil if the credentials cannot be extracted.
func extractCred(c net.Conn) *PeerCred {
	uc, ok := c.(*net.UnixConn)
	if !ok {
		return nil
	}

	raw, err := uc.SyscallConn()
	if err != nil {
		return nil
	}

	var cred *PeerCred
	raw.Control(func(fd uintptr) {
		ucred, err := unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
		if err != nil {
			return
		}
		startTime, err := readProcStartTime(int(ucred.Pid))
		if err != nil {
			startTime = 0
		}
		cred = &PeerCred{
			PID:       int(ucred.Pid),
			UID:       int(ucred.Uid),
			GID:       int(ucred.Gid),
			StartTime: startTime,
		}
	})
	return cred
}

// readProcStartTime reads the process start time from /proc/<pid>/stat.
// This is used to distinguish between different processes that have reused the same PID.
// Returns the start time in clock ticks, or an error if it cannot be read.
func readProcStartTime(pid int) (uint64, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return 0, err
	}
	s := string(data)
	idx := strings.LastIndex(s, ")")
	if idx == -1 {
		return 0, errors.New("malformed /proc/pid/stat")
	}
	// strip off the first two fields (pid and comm) to get to the rest of the fields
	// i.e "1234 (my process) R 5678 ..." -> "R 5678 ..."
	rest := strings.Fields(s[idx+2:])
	if len(rest) <= 19 {
		return 0, errors.New("malformed /proc/pid/stat: too few fields")
	}
	// index 19 (0-based) is the start time
	return strconv.ParseUint(rest[19], 10, 64)
}

func DefaultSocketPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}
	return home + "/.env-guard/env-guard.sock", nil
}
