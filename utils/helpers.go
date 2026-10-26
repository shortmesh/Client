package utils

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strings"
)

func GenerateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	// rand.Read fills the byte slice with cryptographically secure random bytes.
	// It returns an error if it cannot read enough bytes from the OS source.
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// ExtractBracketContent extracts the content inside the first pair of parentheses in the input string.
func ExtractBracketContent(input string) (string, error) {
	start := strings.Index(input, "(")
	end := strings.Index(input, ")")
	if start == -1 || end == -1 || end <= start+1 {
		return "", fmt.Errorf("no content found in brackets")
	}
	content := input[start+1 : end]
	// Remove the "+" character from the content
	content = strings.ReplaceAll(content, "+", "")
	return content, nil
}

// Input validation functions
func SanitizeUsername(username string) (string, error) {
	// Remove any whitespace
	username = strings.TrimSpace(username)

	// Username should be 3-32 characters and contain only letters, numbers, and underscores
	validUsername := regexp.MustCompile(`^[a-zA-Z0-9_]{3,32}$`)
	if !validUsername.MatchString(username) {
		return "", fmt.Errorf("username must be 3-32 characters and contain only letters, numbers, and underscores")
	}

	return username, nil
}

func DeleteFilesWithPattern(dirPath, pattern string) error {
	// Construct the full pattern for filepath.Glob
	fullPattern := filepath.Join(dirPath, pattern)

	// Use filepath.Glob to find matching files
	files, err := filepath.Glob(fullPattern)
	if err != nil {
		return fmt.Errorf("error finding files with pattern %s: %w", fullPattern, err)
	}

	// Loop through the found files and remove each one
	for _, file := range files {
		err := os.Remove(file)
		if err != nil {
			// Log the error but continue with the next file
			slog.Error(err.Error())
			debug.PrintStack()
			return err
		} else {
			slog.Debug("Successfully deleted file", "filename", file)
		}
	}

	return nil
}

func ReplacePlaceholders(format string, values ...string) string {
	result := format
	for _, v := range values {
		result = strings.Replace(result, "%s", v, 1)
	}
	return result
}

func HashFileSHA256(filePath string) (string, error) {
	// Open the file.
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	// Ensure the file is closed once the function returns.
	defer file.Close()

	// Create a new SHA256 hash interface. You can use other algorithms like
	// md5.New() or sha512.New() in a similar way by importing the respective package.
	hasher := sha256.New()

	// Use io.Copy to efficiently stream the file data to the hasher.
	// The hasher implements the io.Writer interface.
	if _, err := io.Copy(hasher, file); err != nil {
		return "", fmt.Errorf("error hashing file: %w", err)
	}

	// Get the finalized hash sum as a byte slice. The argument to Sum can be
	// used to append to an existing byte slice, but passing nil works fine.
	hashInBytes := hasher.Sum(nil)

	// Convert the byte slice to a hexadecimal string for human-readable output.
	hashString := hex.EncodeToString(hashInBytes)

	return hashString, nil
}

func ExtractE164Contact(message string) string {
	re := regexp.MustCompile(`\+[1-9]\d{6,14}`)
	return re.FindString(message)
}

func ExtractAllE164ForTagging(input string) []string {
	// Regex pattern: matches '@' followed by '+' and 1 to 15 digits
	re := regexp.MustCompile(`@(\+\d{1,15})`)

	// FindAllStringSubmatch with -1 finds all occurrences
	matches := re.FindAllStringSubmatch(input, -1)

	var results []string
	for _, match := range matches {
		if len(match) > 1 {
			results = append(results, match[1]) // Append the captured group (+ and digits)
		}
	}

	return results
}

func MatchesTemplate(template, input string) bool {
	placeholder := "{{.}}"

	// Split the template by the placeholder
	parts := strings.Split(template, placeholder)
	if len(parts) == 1 {
		// If no placeholder is found, check for an exact match
		return input == template
	}

	// Build a dynamic regex pattern
	var sb strings.Builder
	sb.WriteString("^")
	for i, part := range parts {
		// QuoteMeta escapes any special regex characters in the template text
		sb.WriteString(regexp.QuoteMeta(part))

		// Insert a general wildcard (.*) where "{{.}}" was located
		if i < len(parts)-1 {
			sb.WriteString(`.*`)
		}
	}
	sb.WriteString("$")

	// Compile and match against the input string
	re, err := regexp.Compile(sb.String())
	if err != nil {
		return false
	}

	return re.MatchString(input)
}

// ExtractTemplateValue checks if the input matches the template and returns
// the value placed in "{{.}}". Returns an empty string if it's not a match.
func ExtractTemplateValue(template, input string) string {
	placeholder := "{{.}}"

	// Split the template by the placeholder
	parts := strings.Split(template, placeholder)
	if len(parts) == 1 {
		if input == template {
			return "" // Template has no placeholder, but matched text exactly
		}
		return ""
	}

	// Build a dynamic regex pattern with capturing groups around the placeholder
	var sb strings.Builder
	sb.WriteString("^")
	for i, part := range parts {
		// QuoteMeta escapes special regex characters in the template text
		sb.WriteString(regexp.QuoteMeta(part))

		// Insert a capturing group (.*) where "{{.}}" was located
		if i < len(parts)-1 {
			sb.WriteString(`(.*)`)
		}
	}
	sb.WriteString("$")

	// Compile and match against the input string
	re, err := regexp.Compile(sb.String())
	if err != nil {
		return ""
	}

	// FindStringSubmatch returns the full match followed by captured groups
	match := re.FindStringSubmatch(input)
	if len(match) > 1 {
		return match[1] // Returns the value captured by the first (.*)
	}

	return "" // Returns blank if no match
}
