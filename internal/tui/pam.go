package tui

import (
	"fmt"
	"os/user"

	"github.com/msteinert/pam"
)

var verifyRootPassword = pamAuthenticate

func pamAuthenticate(password string) error {
	current, err := user.Current()
	if err != nil {
		return fmt.Errorf("get current user: %w", err)
	}

	h, err := pam.StartFunc("sudo", current.Username, func(s pam.Style, msg string) (string, error) {
		if s == pam.PromptEchoOff {
			return password, nil
		}
		return "", fmt.Errorf("unexpected PAM message: %s (style=%d)", msg, s)
	})
	if err != nil {
		return fmt.Errorf("PAM start: %w", err)
	}

	if err := h.Authenticate(0); err != nil {
		return fmt.Errorf("PAM auth: %w", err)
	}
	return nil
}
