package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

func isCloudflareAccessRedirect(resp *http.Response) (bool, string) {
	if resp.StatusCode != http.StatusOK {
		return false, ""
	}

	domain := resp.Header.Get("CF-Access-Domain")
	contentType := resp.Header.Get("Content-Type")
	server := resp.Header.Get("Server")

	if domain != "" && contentType == "text/html" && server == "cloudflare" {
		return true, domain
	}

	return false, ""
}

func cloudflaredAccessToken(domain string) (string, bool, error) {
	// try get token
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var b bytes.Buffer
	cmd := exec.CommandContext(ctx, "cloudflared", "access", "token", "-app="+domain)
	cmd.Stderr = &b
	cmd.Stdout = &b
	err := cmd.Run()
	if err == nil {
		token := b.String()
		if strings.HasPrefix(token, "Unable to find") {
			// no token exists, need to log in
			return "", false, nil
		}
		// success, we have a pre-existing token
		return b.String(), true, nil

	}

	var reterr *exec.ExitError
	if !errors.As(err, &reterr) {
		// failed to even run cloudflared - maybe it isn't installed
		return "", false, fmt.Errorf("failed to run cloudflared: %w", err)
	}

	if reterr.ExitCode() != 1 {
		// we expect 1 for "unable to find token"
		return "", false, fmt.Errorf("cloudflared returned exit code: %d: %s", reterr.ExitCode(), b.String())
	}

	// no token exists, need to log in
	return "", false, nil
}

// urlWriter writes any URLs found to stderr with emojis to make it easy to spot
type urlWriter struct{}

var urlRegex = regexp.MustCompile(`(https://[^\s]+)`)

func (w *urlWriter) Write(b []byte) (int, error) {
	parts := urlRegex.FindSubmatch(b)
	if len(parts) == 2 {
		log.Printf("👉  %s 👈", string(parts[1]))
	}
	return len(b), nil
}

func cloudflaredAccessLogin(domain string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "cloudflared", "access", "login", domain)

	w := urlWriter{}
	cmd.Stderr = &w
	cmd.Stdout = &w
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to run cloudflared: %w", err)
	}
	// success, we can call cloudflared access token now
	return nil
}

func getAccessToken(domain string) (string, error) {
	token, ok, err := cloudflaredAccessToken(domain)
	if err != nil {
		return "", err
	}
	if ok {
		return token, nil
	}

	if err := cloudflaredAccessLogin(domain); err != nil {
		return "", err
	}

	token, ok, err = cloudflaredAccessToken(domain)
	if err != nil {
		return "", err
	}
	if ok {
		return token, nil
	}
	return "", errors.New("failed to retrieve token despite successful login")
}
