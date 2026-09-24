package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"issueCLI/github"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

type issueRequest struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

var token string

func main() {
	token = os.Getenv("GITHUB_TOKEN")
	if token == "" {
		fmt.Println("Failed to get token from env")
		return
	}

	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: gh-issue <command> [arguments]")
		fmt.Fprintln(os.Stderr, "commands: create, read, update, close")
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "create":
		err := createIssue(args)
		if err != nil {
			fmt.Println(err)
		}
	case "read":
		err := readIssue(args)
		if err != nil {
			fmt.Println(err)
		}
	case "update":
		err := updateIssue(args)
		if err != nil {
			fmt.Println(err)
		}
	case "close":
		err := closeIssue(args)
		if err != nil {
			fmt.Println(err)
		}
	}
}

func createIssue(args []string) error {
	fs := flag.NewFlagSet("create", flag.ExitOnError)
	fs.Parse(args)

	positional := fs.Args()
	if len(positional) < 1 {
		return fmt.Errorf("usage: gh-issue create owner/repo")
	}
	ownerRepo := positional[0]
	split := strings.SplitN(ownerRepo, "/", 2)
	if len(split) < 2 {
		return fmt.Errorf("invalid owner/repo format: %q", ownerRepo)
	}
	owner := split[0]
	repo := split[1]
	currentIssue, err := openEditor(nil)
	if err != nil {
		return err
	}
	title, body, err := parseIssue(currentIssue)
	if err != nil {
		return err
	}
	resp, err := doRequest(http.MethodPost, "https://api.github.com/repos/"+owner+"/"+repo+"/issues", issueRequest{title, body}, token)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusCreated {
		fmt.Println("Success to create new issue")
	} else {
		return fmt.Errorf("Error status code %d", resp.StatusCode)
	}

	respIssue := github.Issue{}

	err = json.NewDecoder(resp.Body).Decode(&respIssue)
	if err != nil {
		return err
	} else {
		fmt.Println("Issue number #", respIssue.Number)
	}

	return nil
}

func readIssue(args []string) error {
	fs := flag.NewFlagSet("read", flag.ExitOnError)
	fs.Parse(args)
	positional := fs.Args()
	if len(positional) < 2 {
		return fmt.Errorf("usage: gh-issue read owner/repo <num>")
	}
	ownerRepo := positional[0]
	numIssue := positional[1]
	_, err := strconv.Atoi(numIssue)
	if err != nil {
		return fmt.Errorf("invalid number issue format: %q", numIssue)
	}

	split := strings.SplitN(ownerRepo, "/", 2)
	if len(split) < 2 {
		return fmt.Errorf("invalid owner/repo format: %q", ownerRepo)
	}
	owner := split[0]
	repo := split[1]

	resp, err := doRequest(http.MethodGet, "https://api.github.com/repos/"+owner+"/"+repo+"/issues/"+numIssue, nil, token)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Error with code %d", resp.StatusCode)
	}

	respIssue := github.Issue{}

	err = json.NewDecoder(resp.Body).Decode(&respIssue)
	if err != nil {
		return err
	} else {
		fmt.Println(respIssue.Title)
		fmt.Println(respIssue.Body)
	}

	return nil
}

func updateIssue(args []string) error {
	fs := flag.NewFlagSet("update", flag.ExitOnError)
	fs.Parse(args)

	positional := fs.Args()
	if len(positional) < 2 {
		return fmt.Errorf("usage: gh-issue update owner/repo <num>")
	}
	ownerRepo := positional[0]
	numIssue := positional[1]
	_, err := strconv.Atoi(numIssue)
	if err != nil {
		return fmt.Errorf("invalid number issue format: %q", numIssue)
	}

	split := strings.SplitN(ownerRepo, "/", 2)
	if len(split) < 2 {
		return fmt.Errorf("invalid owner/repo format: %q", ownerRepo)
	}
	owner := split[0]
	repo := split[1]

	resp, err := doRequest(http.MethodGet, "https://api.github.com/repos/"+owner+"/"+repo+"/issues/"+numIssue, nil, token)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Error with code %d", resp.StatusCode)
	}

	respIssue := github.Issue{}

	err = json.NewDecoder(resp.Body).Decode(&respIssue)
	var data []byte
	markdownText := fmt.Sprintf("%s\n\n%s", respIssue.Title, respIssue.Body)
	if err != nil {
		return err
	} else {
		data = []byte(markdownText)
	}

	currentIssue, err := openEditor(data)
	if err != nil {
		return err
	}

	title, body, err := parseIssue(currentIssue)
	if err != nil {
		return err
	}

	resp, err = doRequest(http.MethodPatch, "https://api.github.com/repos/"+owner+"/"+repo+"/issues/"+numIssue, issueRequest{title, body}, token)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Error with status code: %d", resp.StatusCode)
	}

	respIssue = github.Issue{}

	err = json.NewDecoder(resp.Body).Decode(&respIssue)
	if err != nil {
		return err
	} else {
		fmt.Println("Update issue number #", respIssue.Number)
	}
	return nil
}

func closeIssue(args []string) error {
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
			return nil, fmt.Errorf("failed to wtite data to temporary file: %w", err)
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
		return nil, fmt.Errorf("Failed to run editor: %w", err)
	}

	issueData, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("Failed to read data from file: %w", err)
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

func doRequest(method, url string, body interface{}, token string) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		return nil, fmt.Errorf("Failed to create new request %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Failed to send new reques %w", err)
	}

	return res, nil
}
