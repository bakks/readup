package main

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/creack/pty"
)

// readup is a simple utility for keeping a README file up to date
// with command output. Given a README file, it looks for code blocks
// surrounded with '```' that start '> [command]' on the first line.
// It then runs the command and replaces the code block with the
// output of the command (except for the command itself).

// Split s into lines, indent each line 2 spaces and color it with
// the grey color.
func greyFormat(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = fmt.Sprintf("  \x1b[90m%s\x1b[0m", line)
		}
	}
	return strings.Join(lines, "\n")
}

// Read in a string which is the output of calling diff,
// color every line that starts with '<' red, and every line
// that starts with '>' green.
func diffFormat(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "<") || strings.HasPrefix(line, "-") {
			lines[i] = fmt.Sprintf("\x1b[31m%s\x1b[0m", line)
		} else if strings.HasPrefix(line, ">") || strings.HasPrefix(line, "+") {
			lines[i] = fmt.Sprintf("\x1b[32m%s\x1b[0m", line)
		}
	}
	return strings.Join(lines, "\n")
}

// execCommand() is a helper function that runs a command in a PTY
// and returns the output.
func execCommand(cmd []string, print bool) (string, error) {
	if print {
		fmt.Printf("Running: %s\n", strings.Join(cmd, " "))
	}

	args := []string{}
	if len(cmd) > 1 {
		args = cmd[1:]
	}

	command := exec.Command(cmd[0], args...)

	// copy PATH env var from current process
	command.Env = append(os.Environ(), "PATH="+os.Getenv("PATH"))

	winSize := &pty.Winsize{Rows: 40, Cols: 80}
	ptyFile, err := pty.StartWithSize(command, winSize)
	if err != nil {
		return "", err
	}
	defer ptyFile.Close()

	var out []byte
	buf := make([]byte, 1024)
	for {
		n, err := ptyFile.Read(buf)
		if err != nil && err != io.EOF {
			return "", err
		}
		if n == 0 {
			break
		}
		out = append(out, buf[:n]...)
	}

	output := string(out)
	output = strings.Replace(output, "\r", "", -1)

	if print {
		fmt.Printf("Output:\n%s", greyFormat(output))
	}
	return output, nil
}

// readup() is the main function that reads the README file, finds
// the code blocks, looks for a '> [command]' on the first line,
// and if it finds it, executes the command and replaces the code
// block with the output.
func readup(filename string) (string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer file.Close()

	var lines []string
	var inCodeBlock bool
	var codeBlock []string

	// Read the file line by line
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		lines = append(lines, line)

		// If we're in a code block, append the line to the code block
		if inCodeBlock {
			codeBlock = append(codeBlock, line)
		}

		// If we're not in a code block, check if this is the start
		// of a code block
		if !inCodeBlock && strings.HasPrefix(line, "```") {
			inCodeBlock = true
			codeBlock = []string{line}
			continue
		}

		// If we're in a code block, check if this is the end of
		// a code block
		if inCodeBlock && strings.HasPrefix(line, "```") {
			inCodeBlock = false

			// If the first line of the code block starts with
			// '> ', then we have a command
			if strings.HasPrefix(codeBlock[1], "> ") {
				codeBlockCommand := strings.Split(codeBlock[1][2:], " ")
				codeBlockOutput, err := execCommand(codeBlockCommand, true)
				if err != nil {
					return "", err
				}

				blockStart := len(lines) - len(codeBlock) + 2

				// Replace the code block with the output of the command
				lines = lines[:blockStart]
				lines = append(lines, codeBlockOutput)
				lines = append(lines, "```")
			}
		}
	}

	return strings.Join(lines, "\n"), nil
}

func writeFile(filename, content string) error {
	// Write the lines back to the file
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	fmt.Fprintf(file, "%s", content)

	return nil
}

func writeTempFile(filename, content string) (string, error) {
	// Write the lines back to the file
	file, err := ioutil.TempFile("", "readup")
	if err != nil {
		return "", err
	}
	defer file.Close()

	fmt.Fprintf(file, "%s", content)

	return file.Name(), nil
}

func main() {
	filename := ""

	if len(os.Args) != 2 {
		filename = "./README.md"
	} else {
		filename = os.Args[1]
	}

	content, err := readup(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
		os.Exit(1)
	}

	tmpName, err := writeTempFile(filename, content)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
		os.Exit(1)
	}

	diffOut, err := execCommand([]string{"diff", "-u", filename, tmpName}, false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
		os.Exit(1)
	}

	fmt.Println(diffFormat(diffOut))

	// Ask the user to confirm whether they want to update the file
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Update file? [y/N] ")
	text, _ := reader.ReadString('\n')

	if strings.ToLower(strings.TrimSpace(text)) != "y" {
		os.Exit(0)
	}

	// copy the temp file to the original file
	err = exec.Command("cp", tmpName, filename).Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
		os.Exit(1)
	}

	// remove the temp file
	err = os.Remove(tmpName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
		os.Exit(1)
	}

	os.Exit(0)
}
