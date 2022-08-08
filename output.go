package main

import (
	"errors"
	"fmt"
	"github.com/fatih/color"
	"regexp"
)

//go:generate go run -modfile tools.mod github.com/dmarkham/enumer -type Hint -trimprefix Hint -json

// TODO(steve): inline color so we can enumer it for config; https://github.com/afbjorklund/go-termbg

type Hint int

const (
	HintInfo Hint = iota
	HintWarn
	HintError
	HintSend
	HintSendTls
	HintSendQ
	HintSendTlsQ
	HintSendChunk
	HintRecv
	HintRecvTls
	HintRecvQ
	HintRecvTlsQ
	HintRecvChunk
	HintAccept
	HintReject
	HintDefer
)

type Style struct {
	Tag   string
	Color *color.Color
}

func Error(err error) {
	Errorf("%s\n", err.Error())
}

func Errorf(msg string, args ...interface{}) {
	color.HiRed(msg, args...)
}

func Fatal(err error) {
	if !errors.Is(err, nil) {
		Error(err)
	}
	var ex ExitError
	if errors.As(err, &ex) {
		Exit(ex.exit)
	}
	Exit(ExitOther)
}

func Warn(msg string) {
	Warnf("%s\n", msg)
}

func Warnf(msg string, args ...interface{}) {
	color.HiYellow(msg, args...)
}

var lineEndRE = regexp.MustCompile(`\r?\n`)

var acceptRe = regexp.MustCompile(`^[0-36-9][0-9]{2}`)
var deferRe = regexp.MustCompile(`^4[0-9]{2}`)
var rejectRe = regexp.MustCompile(`^5[0-9]{2}`)

func (c *Client) Message(hint Hint, msg string) {
	if c.tls {
		switch hint {
		case HintSend:
			hint = HintSendTls
		case HintSendQ:
			hint = HintSendTlsQ
		case HintRecv:
			hint = HintRecvTls
		case HintRecvQ:
			hint = HintRecvTlsQ
		}
	}
	c.config.Message(hint, msg)
}

func (config Config) Message(hint Hint, msg string) {
	showHint := true
	switch hint {
	case HintInfo:
		if config.HideInfo {
			return
		}
		if config.NoInfoHints {
			showHint = false
		}
	case HintSend, HintSendTls, HintSendQ, HintSendTlsQ, HintSendChunk:
		if config.HideSend {
			return
		}
		if config.NoSendHints {
			showHint = false
		}
	case HintRecv, HintRecvTls, HintRecvQ, HintRecvTlsQ, HintRecvChunk:
		if config.HideReceive {
			return
		}
		if config.NoReceiveHints {
			showHint = false
		}
	}

	t := config.Colors[hint]
	lines := lineEndRE.Split(msg, -1)
	for _, line := range lines {
		textColor := t.Color
		if hint == HintRecv || hint == HintRecvTls {
			switch {
			case acceptRe.MatchString(line):
				textColor = config.Colors[HintAccept].Color
			case deferRe.MatchString(line):
				textColor = config.Colors[HintDefer].Color
			case rejectRe.MatchString(line):
				textColor = config.Colors[HintReject].Color
			}
		}
		if showHint {
			_, _ = textColor.Printf("%s %s\n", t.Tag, line)
		} else {
			_, _ = textColor.Printf("%s\n", line)
		}
	}
}

func (c *Client) Messagef(hint Hint, msg string, args ...interface{}) {
	c.config.Messagef(hint, msg, args...)
}

func (config Config) Messagef(hint Hint, msg string, args ...interface{}) {
	config.Message(hint, fmt.Sprintf(msg, args...))
}
