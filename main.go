package main

import (
	"bytes"
	"flag"
	"fmt"
	"issueCLI/github"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

var token string

func main() {
	token = os.Getenv("GITHUB_TOKEN")
	if token == "" {
		fmt.Println("failed to get token from env")
		return
	}

	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: gh-issue <command> [arguments]")
		fmt.Fprintln(os.Stderr, "commands: create, read, update, close")
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	client := github.NewClient(token)

	switch cmd {
	case "create":
		err := createIssue(args, client)
		if err != nil {
			fmt.Println(err)
		}
	case "read":
		err := readIssue(args, client)
		if err != nil {
			fmt.Println(err)
		}
	case "update":
		err := updateIssue(args, client)
		if err != nil {
			fmt.Println(err)
		}
	case "close":
		err := closeIssue(args, client)
		if err != nil {
			fmt.Println(err)
		}
	}
}

func createIssue(args []string, client *github.Client) error {
	fs := flag.NewFlagSet("create", flag.ExitOnError)
	fs.Parse(args)

	positional := fs.Args()
	owner, repo, _, err := parseArgs(positional, "create", false)
	if err != nil {
		return err
	}
	currentIssue, err := openEditor(nil)
	if err != nil {
		return err
	}
	title, body, err := parseIssue(currentIssue)
	if err != nil {
		return err
	}

	issue, err := client.CreateIssue(owner, repo, title, body)
	if err != nil {
		return err
	}

	fmt.Printf("issue #%d created successfully\n", issue.Number)

	return nil
}

func readIssue(args []string, client *github.Client) error {
	fs := flag.NewFlagSet("read", flag.ExitOnError)
	fs.Parse(args)
	positional := fs.Args()
	owner, repo, numIssue, err := parseArgs(positional, "read", true)
	if err != nil {
		return err
	}

	issue, err := client.ReadIssue(owner, repo, numIssue)
	if err != nil {
		return err
	}
	fmt.Printf("Issue #%d\n%v\n%v\n", issue.Number, issue.Title, issue.Body)

	return nil
}

func updateIssue(args []string, client *github.Client) error {
	fs := flag.NewFlagSet("update", flag.ExitOnError)
	fs.Parse(args)

	positional := fs.Args()
	owner, repo, numIssue, err := parseArgs(positional, "update", true)
	if err != nil {
		return err
	}

	issue, err := client.ReadIssue(owner, repo, numIssue)
	if err != nil {
		return err
	}

	markdownText := fmt.Sprintf("%s\n%s", issue.Title, issue.Body)
	data := []byte(markdownText)

	currentIssue, err := openEditor(data)
	if err != nil {
		return err
	}

	title, body, err := parseIssue(currentIssue)
	if err != nil {
		return err
	}

	issue, err = client.UpdateIssue(owner, repo, numIssue, title, body)
	if err != nil {
		return err
	}
	fmt.Printf("issue #%d successfully updated", issue.Number)

	return nil
}

func closeIssue(args []string, client *github.Client) error {
	fs := flag.NewFlagSet("close", flag.ExitOnError)
	fs.Parse(args)

	positional := fs.Args()
	owner, repo, numIssue, err := parseArgs(positional, "close", true)
	if err != nil {
		return err
	}

	err = client.CloseIssue(owner, repo, numIssue)
	if err != nil {
		return err
	}

	fmt.Printf("Issue #%d closed", numIssue)

	return nil
}

func openEditor(data []byte) ([]byte, error) {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		if runtime.GOOS == "windows" {
			editor = "code"
		} else {
			editor = "nano"
		}
	}

	tempFile, err := os.CreateTemp("", "temp_file_*.md")
	if err != nil {
		return nil, fmt.Errorf("failed to create a temporary file: %w", err)
	}

	filePath := tempFile.Name()

	defer os.Remove(filePath)
	if data != nil {
		_, err := tempFile.Write(data)
		if err != nil {
			return nil, fmt.Errorf("failed to write data to temporary file: %w", err)
		}
	}

	tempFile.Close()

	var commands []string
	if editor == "code" {
		commands = []string{filePath, "--wait"}
	} else {
		commands = []string{filePath}
	}

	cmd := exec.Command(editor, commands...)

	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to run editor: %w", err)
	}

	issueData, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read data from file: %w", err)
	}

	return issueData, nil
}

func parseIssue(data []byte) (title, body string, err error) {
	if len(data) == 0 {
		return "", "", fmt.Errorf("empty data")
	}

	line, rest, found := bytes.Cut(data, []byte("\n"))

	if found {
		return string(line), string(rest), nil
	}

	return string(data), "", nil
}

func parseArgs(positional []string, method string, needNumber bool) (string, string, int, error) {
	numArgs := 0
	if needNumber {
		numArgs = 2
	} else {
		numArgs = 1
	}
	usage := fmt.Sprintf("usage: gh-issue %v owner/repo", method)
	if needNumber {
		usage += " <num>"
	}
	if len(positional) != numArgs {
		return "", "", 0, fmt.Errorf(usage)
	}
	ownerRepo := positional[0]
	var num int
	var err error
	if numArgs == 2 {
		numIssueStr := positional[1]
		num, err = strconv.Atoi(numIssueStr)
		if err != nil {
			return "", "", 0, fmt.Errorf("invalid number issue format: %q", numIssueStr)
		}
	}

	split := strings.SplitN(ownerRepo, "/", 2)
	if len(split) < 2 {
		return "", "", 0, fmt.Errorf("invalid owner/repo format: %q", ownerRepo)
	}

	owner := split[0]
	repo := split[1]
	return owner, repo, num, nil
}
