package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	"golang.org/x/term"
)

// PromptForInput prompts the user for visible input (like username)
func PromptForInput(prompt string) string {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
}

// PromptForPassword prompts the user for masked password input
func PromptForPassword(prompt string) string {
	fmt.Print(prompt)
	password, _ := term.ReadPassword(int(syscall.Stdin))
	fmt.Println() // New line after password entry
	return string(password)
}

// PromptForPaths prompts the user to enter multiple paths
// User presses enter with empty input to finish
func PromptForPaths() []string {
	var paths []string
	fmt.Println("Enter library paths (press Enter on empty line to finish):")
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("Path: ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input == "" {
			break
		}

		paths = append(paths, input)
	}

	return paths
}

// PromptForConfirmation prompts the user for yes/no confirmation
func PromptForConfirmation(prompt string) bool {
	fmt.Printf("%s (y/N): ", prompt)
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))

	return input == "y" || input == "yes"
}
