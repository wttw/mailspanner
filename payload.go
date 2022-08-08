package main

import (
	"bytes"
	"math/rand"
	"os"
	"regexp"
	"strings"
	"text/template"
	"time"
)

type MessageVars struct {
	Cookie      string
	FromAddress string
	ToAddress   string
	Date        string
	MessageID   string
	Version     string
	NewHeaders  string
	Body        string
}

func MakePayload(c Config) (string, error) {
	if c.NoDataFixup {
		return c.Data, nil
	}

	// Make a random cookie
	const letters = "abcdefghijklmnopqrstuvwxyz"
	var sb strings.Builder
	sb.Grow(8)
	for i := 0; i < 8; i++ {
		sb.WriteByte(letters[rand.Intn(len(letters))]) //nolint:gosec
	}

	host, err := os.Hostname()
	if err != nil {
		// :shrug:
		Warnf("failed to find system hostname: %v", err)
		host = "hostname.failed.invalid"
	}

	vars := MessageVars{
		Cookie:      sb.String(),
		FromAddress: c.From,
		ToAddress:   strings.Join(c.To, ", "),
		Date:        time.Now().Format(time.RFC822),
		MessageID:   sb.String() + "@" + host,
		Version:     Version,
		NewHeaders:  strings.Join(c.AdditionalHeaders, "\n"),
		Body:        c.Body,
	}

	// TODO(steve): replace cookie in body
	// FIXME(steve): Set headers if needed

	replacer := strings.NewReplacer(
		`\n`, "\n",
		"%FROM_ADDRESS%", "{{ .FromAddress }}",
		"%TO_ADDRESS%", "{{ .ToAddress }}",
		"%DATE%", "{{ .Date }}",
		"%MESSAGEID%", "{{ .MessageID }}",
		"%VERSION%", "{{ .Version }}",
		"%SWAKS_VERSION%", "{{ .Version }}",
		"%MAILSPANNER_VERSION%", "{{ .Version }}",
		"%NEW_HEADERS%", "{{ .NewHeaders }}",
		"%BODY%", "{{ .Body }}",
		"%NEWLINE", "\r\n",
	)
	tpl, err := template.New("data").Parse(replacer.Replace(c.Data))
	if err != nil {
		return "", Fatalf(ExitFlags, "failed to compile template for data: %s", err)
	}
	var buff bytes.Buffer
	err = tpl.Execute(&buff, vars)
	if err != nil {
		return "", Fatalf(ExitFlags, "failed to execute template for data: %s", err)
	}
	crlfre := regexp.MustCompile(`\r?\n`)
	return crlfre.ReplaceAllString(buff.String(), "\r\n"), nil
}
