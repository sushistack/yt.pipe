package cli

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// promptString prompts the user for a string value. If the user enters nothing,
// defaultVal is returned. Leading/trailing whitespace is trimmed.
func promptString(r io.Reader, w io.Writer, label, defaultVal string) (string, error) {
	if defaultVal != "" {
		fmt.Fprintf(w, "%s [%s]: ", label, defaultVal)
	} else {
		fmt.Fprintf(w, "%s: ", label)
	}

	scanner := bufio.NewScanner(r)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", fmt.Errorf("init wizard: %w", err)
		}
		return defaultVal, nil
	}

	val := strings.TrimSpace(scanner.Text())
	if val == "" {
		return defaultVal, nil
	}
	return val, nil
}

// promptSecret prompts the user for a secret value (e.g., API key).
// For MVP this is a simple line read without echo suppression.
func promptSecret(r io.Reader, w io.Writer, label string) (string, error) {
	fmt.Fprintf(w, "%s: ", label)

	scanner := bufio.NewScanner(r)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", fmt.Errorf("init wizard: %w", err)
		}
		return "", nil
	}

	return strings.TrimSpace(scanner.Text()), nil
}

// promptSelect shows a numbered list of options and prompts the user to pick one.
// defaultIdx is the 1-based index used when the user enters nothing.
// Returns the selected option string.
func promptSelect(r io.Reader, w io.Writer, label string, options []string, defaultIdx int) (string, error) {
	if len(options) == 0 {
		return "", fmt.Errorf("init wizard: no options provided")
	}
	if defaultIdx < 1 || defaultIdx > len(options) {
		return "", fmt.Errorf("init wizard: default index %d out of range [1, %d]", defaultIdx, len(options))
	}

	fmt.Fprintf(w, "%s:\n", label)
	for i, opt := range options {
		marker := "  "
		if i+1 == defaultIdx {
			marker = "* "
		}
		fmt.Fprintf(w, "  %s%d) %s\n", marker, i+1, opt)
	}

	scanner := bufio.NewScanner(r)
	for {
		fmt.Fprintf(w, "Choose [%d]: ", defaultIdx)
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return "", fmt.Errorf("init wizard: %w", err)
			}
			return options[defaultIdx-1], nil
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			return options[defaultIdx-1], nil
		}

		choice, err := strconv.Atoi(input)
		if err != nil || choice < 1 || choice > len(options) {
			fmt.Fprintf(w, "Invalid choice. Please enter a number between 1 and %d.\n", len(options))
			continue
		}
		return options[choice-1], nil
	}
}

// promptConfirm prompts the user for a yes/no confirmation.
// defaultYes controls the default when the user presses Enter without typing.
func promptConfirm(r io.Reader, w io.Writer, label string, defaultYes bool) (bool, error) {
	hint := "y/N"
	if defaultYes {
		hint = "Y/n"
	}
	fmt.Fprintf(w, "%s [%s]: ", label, hint)

	scanner := bufio.NewScanner(r)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return false, fmt.Errorf("init wizard: %w", err)
		}
		return defaultYes, nil
	}

	input := strings.TrimSpace(strings.ToLower(scanner.Text()))
	switch input {
	case "":
		return defaultYes, nil
	case "y", "yes":
		return true, nil
	case "n", "no":
		return false, nil
	default:
		return defaultYes, nil
	}
}
