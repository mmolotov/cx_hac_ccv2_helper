package ui

import (
	"fmt"

	"github.com/manifoldco/promptui"
)

// PromptPassword prompts the user for a password with masked input.
func PromptPassword(label string) (string, error) {
	p := promptui.Prompt{
		Label: label,
		Mask:  '*',
	}
	val, err := p.Run()
	if err != nil {
		return "", fmt.Errorf("password prompt: %w", err)
	}
	return val, nil
}

// PromptOTP prompts the user for a TOTP code.
func PromptOTP(label string) (string, error) {
	p := promptui.Prompt{
		Label: label,
		Validate: func(s string) error {
			if len(s) < 6 {
				return fmt.Errorf("OTP must be at least 6 digits")
			}
			return nil
		},
	}
	val, err := p.Run()
	if err != nil {
		return "", fmt.Errorf("OTP prompt: %w", err)
	}
	return val, nil
}

// PromptText prompts the user for a plain text value.
func PromptText(label string) (string, error) {
	p := promptui.Prompt{
		Label: label,
	}
	val, err := p.Run()
	if err != nil {
		return "", fmt.Errorf("text prompt: %w", err)
	}
	return val, nil
}

// PromptTextDefault prompts the user for a plain text value with a pre-filled default.
func PromptTextDefault(label, defaultVal string) (string, error) {
	p := promptui.Prompt{
		Label:   label,
		Default: defaultVal,
	}
	val, err := p.Run()
	if err != nil {
		return "", fmt.Errorf("text prompt: %w", err)
	}
	return val, nil
}

// PromptConfirm asks a yes/no question. Returns false on ErrAbort (Ctrl+C or N).
func PromptConfirm(label string) (bool, error) {
	p := promptui.Prompt{
		Label:     label,
		IsConfirm: true,
	}
	_, err := p.Run()
	if err == promptui.ErrAbort {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("confirm prompt: %w", err)
	}
	return true, nil
}

// PromptSelect displays a numbered list and returns the selected item.
func PromptSelect(label string, items []string) (string, error) {
	s := promptui.Select{
		Label: label,
		Items: items,
	}
	_, result, err := s.Run()
	if err != nil {
		return "", fmt.Errorf("select prompt: %w", err)
	}
	return result, nil
}
