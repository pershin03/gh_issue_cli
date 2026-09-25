package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const baseURL = "https://api.github.com"

type Issue struct {
	Number    int
	HTMLURL   string `json:"html_url"`
	Title     string
	State     string
	User      *User
	CreatedAt time.Time `json:"created_at"`
	Body      string    // in Markdown format
}

type User struct {
	Login   string
	HTMLURL string `json:"html_url"`
}

type issueRequest struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

type issueClose struct {
	State string `json:"state"`
}

type Client struct {
	token      string
	httpClient http.Client
}

func NewClient(token string) *Client {
	return &Client{
		token:      token,
		httpClient: http.Client{Timeout: 10 * time.Second},
	}
}

func issueURL(owner, repo string, number int) string {
	return fmt.Sprintf("%s/repos/%s/%s/issues/%d", baseURL, owner, repo, number)
}

func issuesURL(owner, repo string) string {
	return fmt.Sprintf("%s/repos/%s/%s/issues", baseURL, owner, repo)
}

func (c *Client) CreateIssue(owner, repo, title, body string) (*Issue, error) {
	url := issuesURL(owner, repo)
	resp, err := c.doRequest(http.MethodPost, url, issueRequest{title, body})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("error status code %d", resp.StatusCode)
	}

	respIssue := Issue{}

	err = json.NewDecoder(resp.Body).Decode(&respIssue)
	if err != nil {
		return nil, err
	}

	return &respIssue, nil
}

func (c *Client) ReadIssue(owner, repo string, number int) (*Issue, error) {
	url := issueURL(owner, repo, number)
	resp, err := c.doRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error with code %d", resp.StatusCode)
	}

	respIssue := Issue{}

	err = json.NewDecoder(resp.Body).Decode(&respIssue)
	if err != nil {
		return nil, err
	}

	return &respIssue, nil
}

func (c *Client) UpdateIssue(owner, repo string, number int, title, body string) (*Issue, error) {
	url := issueURL(owner, repo, number)
	resp, err := c.doRequest(http.MethodPatch, url, issueRequest{title, body})
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error with code %d", resp.StatusCode)
	}

	respIssue := Issue{}

	err = json.NewDecoder(resp.Body).Decode(&respIssue)
	if err != nil {
		return nil, err
	}

	return &respIssue, nil
}

func (c *Client) CloseIssue(owner, repo string, number int) error {
	url := issueURL(owner, repo, number)
	resp, err := c.doRequest(http.MethodPatch, url, issueClose{"closed"})
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error with status code: %d", resp.StatusCode)
	}

	return nil
}

func (c *Client) doRequest(method, url string, body interface{}) (*http.Response, error) {
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
		return nil, fmt.Errorf("failed to create new request %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)

	client := c.httpClient

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send new request %w", err)
	}

	return res, nil
}
