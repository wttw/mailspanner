package main

import (
	"context"
	"io"
	"net"
	"sort"
	"strings"
)

func send(config Config, payload string) error {
	res := net.Resolver{
		PreferGo: true,
	}
	ctx := context.Background()
	if config.Size == -1 {
		config.Size = len(payload)
	}

	// If the user has given us a server
	if config.Server != "" {
		err, _ := sendToHost(config, config.To, config.Server, false, payload)
		return err
	}

	domains := map[string][]string{}
	for _, email := range config.To {
		domain := emailHost(config, email)
		if domain == "" {
			config.Messagef(HintError, "Recipient '%s' has no hostname", email)
			continue
		}
		domains[domain] = append(domains[domain], email)
	}

	for dom, emails := range domains {
		if len(domains) > 1 {
			config.Messagef(HintInfo, "Delivering to %s...", dom)
		}
		mxes, err := res.LookupMX(ctx, dom)
		if err != nil {
			config.Messagef(HintWarn, "While resolving MX for %s: %v", dom, err)
		}
		if len(mxes) == 0 {
			_, _ = sendToHost(
				config,
				emails,
				net.JoinHostPort(dom, "25"),
				true, payload)
			continue
		}
		sort.SliceStable(mxes, func(i, j int) bool {
			return mxes[i].Pref < mxes[j].Pref
		})
	}

	return nil
}

// Attempt to connect to a hostname:port and deliver a
// message. Returns error and true if it managed to dial,
// false otherwise.
func sendToHost(config Config, recipients []string, addr string, v4only bool, payload string) (error, bool) {
	conn, err := Dial(config, addr, v4only)
	if err != nil {
		return err, false
	}
	client, err := NewClient(config, conn, addr)
	if err != nil {
		return err, true
	}
	err = sendTo(config, recipients, client, payload)
	return err, true
}

func sendTo(config Config, recipients []string, c *Client, payload string) error {
	defer c.Close()

	if err := c.hello(); err != nil {
		return err
	}

	err := c.Mail(config.From)
	if err != nil {
		return err
	}
	for _, addr := range recipients {
		if err = c.Rcpt(addr); err != nil {
			return err
		}
	}
	w, err := c.Data()
	if err != nil {
		return err
	}

	if config.SuppressData {
		_, err = io.Copy(w, strings.NewReader(payload))
		if err != nil {
			return err
		}
	} else {
		lines := strings.SplitAfter(payload, "\n")
		for _, line := range lines {
			if !strings.HasSuffix(line, "\r\n") {
				line += "\r\n"
			}
			c.Message(HintSend, strings.TrimSuffix(line, "\r\n"))
			_, err = io.WriteString(w, line)
			if err != nil {
				return err
			}
		}
	}
	err = w.Close()
	if err != nil {
		return err
	}
	if c.config.DropAfterSend == StageDot {
		return c.drop()
	}
	err = c.Quit()
	if err != nil {
		_ = c.Close()
		return err
	}
	return nil
}
