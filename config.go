package main

//go:generate go run -modfile tools.mod github.com/dmarkham/enumer -type Stage -trimprefix Stage -json

import (
	"encoding/json"
	"github.com/fatih/color"
	flag "github.com/spf13/pflag"
	"io"
	"net"
	"os"
	"os/user"
	"strings"
	"time"
)

const defaultData = `Date: %DATE%\nTo: %TO_ADDRESS%\nFrom: %FROM_ADDRESS%\nSubject: test %DATE%\nMessage-Id: <%MESSAGEID%>\nX-Mailer: mailspanner v%MAILSPANNER_VERSION% github.com/wttw/mailspanner\n%NEW_HEADERS%\n%BODY%\n`

// Stage is a point in the SMTP transaction at which to exit
type Stage int

const (
	StageNone Stage = iota
	StageConnect
	StageBanner // = Connect
	StageFirstHelo
	StageFirstEhlo // = FirstEhlo
	StageXclient
	StageStarttls
	StageTls // = Starttls
	StageHelo
	StageEhlo // = Helo
	StageAuth
	StageMail
	StageFrom // = Mail
	StageRcpt
	StageTo // = Rcpt
	StageData
	StageDot
)

// Config holds the configuration from the commandline
type Config struct {
	Server            string
	Port              string
	CopyRouting       string
	V4                bool
	V6                bool
	To                []string
	From              string
	Helo              string
	QuitAfter         Stage
	DropAfter         Stage
	DropAfterSend     Stage
	Timeout           time.Duration
	Pipeline          bool
	Data              string
	Body              string
	AdditionalHeaders []string
	Headers           []string
	SuppressData      bool
	Timing            bool
	HideReceive       bool
	HideSend          bool
	HideInfo          bool
	NoReceiveHints    bool
	NoSendHints       bool
	NoInfoHints       bool
	Colors            map[Hint]Style
	NoDataFixup       bool
	SendHelo          bool
	Size              int
	SmtpUTF8          bool
	UseStartTLS       bool

	// Values we scan into, then process into what we want
	dump          bool
	quitAfter     string
	dropAfter     string
	dropAfterSend string
	data          string
	body          string
	hideAll       bool
	dumpMail      bool
}

var theme = []struct {
	hint  Hint
	tag   string
	color color.Attribute
}{
	{HintInfo, "===", color.FgWhite},
	{HintWarn, "+++", color.FgHiYellow},
	{HintError, "***", color.FgHiRed},
	{HintSend, " ->", color.FgCyan},
	{HintSendTls, " ~>", color.FgCyan},
	{HintSendQ, "**>", color.FgHiCyan},
	{HintSendTlsQ, "*~>", color.FgHiCyan},
	{HintSendChunk, "  >", color.FgCyan},
	{HintRecv, "<- ", color.FgBlue},
	{HintRecvTls, "<~ ", color.FgBlue},
	{HintRecvQ, "<**", color.FgHiBlue},
	{HintRecvTlsQ, "<~*", color.FgHiBlue},
	{HintRecvChunk, "<  ", color.FgBlue},
	{HintAccept, "<- ", color.FgGreen},
	{HintReject, "<- ", color.FgRed},
	{HintDefer, "<- ", color.FgYellow},
}

func (config *Config) flagSet() *flag.FlagSet {
	fs := flag.NewFlagSet(AppName, flag.ContinueOnError)
	fs.StringVarP(&config.Server, "server", "s", "", "The server[:port] to connect to")
	fs.StringVar(&config.Server, "s", "", "The server[:port] to connect to")
	fs.StringVarP(&config.Port, "port", "p", "", "The port to connect to")
	fs.StringVar(&config.Port, "p", "", "The port to connect to")
	fs.StringVar(&config.CopyRouting, "copy-routing", "", "Choose destination server as though mail were sent to this domain")
	fs.BoolVarP(&config.V4, "4", "4", false, "Force IPv4")
	fs.BoolVarP(&config.V6, "6", "6", false, "Force IPv6")
	fs.StringSliceVarP(&config.To, "to", "t", []string{}, "Comma-separated list of recipient email addresses")
	fs.StringSliceVar(&config.To, "t", []string{}, "Comma-separated list of recipient email addresses")
	fs.StringVarP(&config.From, "from", "f", "", "Envelope sender of email")
	fs.StringVar(&config.From, "f", "", "Envelope sender of email")
	fs.StringVar(&config.Helo, "helo", "", "Value to use for HELO")
	fs.StringVar(&config.Helo, "ehlo", "", "Value to use for HELO")
	fs.DurationVar(&config.Timeout, "timeout", 30*time.Second, "Timeout after this long")
	fs.BoolVar(&config.Pipeline, "pipeline", false, "Use ESMTP pipelining")
	fs.StringVar(&config.Data, "data", defaultData, "Use the argument as the entire contents of DATA")
	fs.StringVar(&config.Body, "body", "This is a test mailing.", "Specify the body of the email")
	fs.BoolVar(&config.dump, "dump", false, "Dump configuration to stdout and exit")
	fs.StringArrayVar(&config.AdditionalHeaders, "add-header", []string{}, "Add header")
	fs.StringArrayVar(&config.AdditionalHeaders, "ah", []string{}, "Add header")
	fs.StringArrayVar(&config.Headers, "header", []string{}, "Set header")
	fs.BoolVar(&config.SuppressData, "suppress-data", false, "Don't display the contents of data")
	fs.BoolVar(&config.Timing, "timing", false, "Display timestamps")
	fs.BoolVar(&config.HideReceive, "hide-receive", false, "Hide the responses received")
	fs.BoolVar(&config.HideReceive, "hr", false, "Hide the responses received")
	fs.BoolVar(&config.HideSend, "hide-send", false, "Hide the commands sent")
	fs.BoolVar(&config.HideSend, "hs", false, "Hide the commands sent")
	fs.BoolVar(&config.HideInfo, "hide-informational", false, "Hide informational messages")
	fs.BoolVar(&config.HideInfo, "hi", false, "Hide informational messages")
	fs.BoolVar(&config.hideAll, "hide-all", false, "Hide all information sent to the terminal")
	fs.BoolVar(&config.hideAll, "ha", false, "Hide all information sent to the terminal")
	fs.BoolVar(&config.dumpMail, "dump-mail", false, "Dump the generated data to stdout and exit")
	fs.StringVarP(&config.quitAfter, "quit-after", "q", "", "Quit after this point")
	fs.StringVar(&config.quitAfter, "quit", "", "Quit after this point")
	fs.StringVar(&config.quitAfter, "q", "", "Quit after this point")
	fs.StringVar(&config.dropAfter, "drop-after", "", "Drop connection at this point")
	fs.StringVar(&config.dropAfter, "da", "", "Drop connection at this point")
	fs.StringVar(&config.dropAfterSend, "drop-after-send", "", "Drop connection after sending response at this point")
	fs.StringVar(&config.dropAfterSend, "das", "", "Drop connection after sending response at this point")
	fs.BoolVar(&config.NoDataFixup, "no-data-fixup", false, "Don't clean up the data section")
	fs.IntVar(&config.Size, "size", 0, "Send SIZE ESMTP option")
	fs.Lookup("size").NoOptDefVal = "-1"
	fs.BoolVar(&config.SmtpUTF8, "smtputf8", false, "Request SMTPUTF8")
	// TODO(steve) no-*-hints
	return fs
}

// ParseFlags parses commandline flags from args (e.g. os.Args()[1:])
func (config *Config) ParseFlags(args []string) error {
	fs := config.flagSet()
	err := fs.Parse(args)
	if err != nil {
		return ExitError{
			err:  err,
			exit: ExitFlags,
		}
	}

	err = config.Normalize()
	if err != nil {
		return err
	}
	err = config.Validate()
	if err != nil {
		return err
	}

	if config.dump {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		encoder.SetEscapeHTML(false)
		_ = encoder.Encode(config)
		Exit(ExitOk)
	}

	return nil
}

// Normalize fixes up a configuration by setting defaults etc.
func (config *Config) Normalize() error {
	// Being vewwy, vewwy quiet
	if config.hideAll {
		config.HideInfo = true
		config.HideReceive = true
		config.HideSend = true
	}

	// Read in things we might get from file or stdin
	var err error
	config.data, err = handleFile("--data", config.Data)
	if err != nil {
		return err
	}
	config.body, err = handleFile("--body", config.Body)
	if err != nil {
		return err
	}

	// Handle the stages we might want to stop after
	config.QuitAfter, err = handleStage("--quit-after", config.quitAfter)
	if err != nil {
		return err
	}
	if config.QuitAfter == StageData || config.QuitAfter == StageDot {
		return Fatalf(ExitFlags, "invalid value for --quit-after: '%s'", config.quitAfter)
	}
	config.DropAfter, err = handleStage("--drop-after", config.dropAfter)
	if err != nil {
		return err
	}
	config.DropAfterSend, err = handleStage("--drop-after-send", config.dropAfterSend)
	if err != nil {
		return err
	}

	if config.From == "<>" {
		config.From = ""
	} else if config.From == "" {
		u, err := user.Current()
		if err != nil {
			return Fatalf(ExitFlags, "failed to retrieve username for --from: %w", err)
		}
		host, err := os.Hostname()
		if err != nil {
			return Fatalf(ExitFlags, "failed to retrieve hostname for --from: %w", err)
		}
		config.From = u.Username + "@" + host
	}

	if config.Helo == "" {
		host, err := os.Hostname()
		if err != nil {
			return Fatalf(ExitFlags, "failed to retrieve hostname for --helo: %w", err)
		}
		config.Helo = host
	}

	if config.Server != "" {
		if !strings.Contains(config.Server, ":") || net.ParseIP(config.Server) != nil {
			config.Server = net.JoinHostPort(config.Server, "25")
		}
	}

	config.Colors = make(map[Hint]Style, len(theme))
	for _, t := range theme {
		config.Colors[t.hint] = Style{
			Tag:   t.tag,
			Color: color.New(t.color),
		}
	}
	return nil
}

func (config *Config) Validate() error {
	if len(config.To) == 0 {
		return Fatalf(ExitFlags, "at least one recipient must be given")
	}
	return nil
}

func handleStage(name, input string) (Stage, error) {
	if input == "" {
		return StageNone, nil
	}
	s, err := StageString(strings.ToLower(input))
	if err != nil {
		return StageNone, Fatalf(ExitFlags, "invalid value for %s: '%s'", name, input)
	}
	switch s {
	case StageBanner:
		s = StageConnect
	case StageFirstEhlo:
		s = StageFirstHelo
	case StageTls:
		s = StageStarttls
	case StageEhlo:
		s = StageHelo
	case StageFrom:
		s = StageMail
	case StageTo:
		s = StageRcpt
	}
	return s, nil
}

func handleFile(name, input string) (string, error) {
	if input == "-" || input == "@-" {
		buff, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", Fatalf(ExitFlags, "while reading stdin for %s: %w", name, err)
		}
		return string(buff), nil
	}
	if strings.HasPrefix(input, "@") {
		input = input[1:]
		if !strings.HasPrefix(input, "@") {
			buff, err := os.ReadFile(input)
			if err != nil {
				return "", Fatalf(ExitFlags, "while reading '%s' for %s: %w", input, name, err)
			}
			return string(buff), nil
		}
	}
	return strings.ReplaceAll(input, `\n`, "\n"), nil
}

func emailHost(config Config, email string) string {
	at := strings.LastIndex(email, "@")
	if at == -1 {
		return ""
	}
	return email[at+1:]
}
