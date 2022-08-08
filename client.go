package main

import (
	"bytes"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/textproto"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	config     Config
	Text       *textproto.Conn
	remoteHost string
	conn       net.Conn
	tls        bool
	didHello   bool
	helloError error
	ext        map[string]string // supported extensions
	auth       []string          // authentication types
	rcpts      []string          // recipients accepted in this session
}

func Dial(config Config, addr string, v4only bool) (net.Conn, error) {
	config.Messagef(HintInfo, "Trying %s...", addr)
	var network string
	switch {
	case config.V4, v4only:
		network = "tcp4"
	case config.V6:
		network = "tcp6"
	default:
		network = "tcp"
	}
	conn, err := net.DialTimeout(network, addr, config.Timeout)

	if err != nil {
		config.Messagef(HintWarn, "Failed to connect to %s: %v", addr, err)
	}
	return conn, err
	//if err != nil {
	//	return &Client{config: config}, err
	//}

	//host, _, _ := net.SplitHostPort(addr)
	//config.Messagef(HintInfo, "Connected to %s.", host)
	//return NewClient(config, conn, host)
}

func NewClient(config Config, conn net.Conn, host string) (*Client, error) {
	config.Messagef(HintInfo, "Connected to %s.", host)
	c := &Client{
		config:     config,
		remoteHost: host,
	}
	c.setConn(conn)
	_ = c.conn.SetDeadline(time.Now().Add(c.config.Timeout))
	defer func(conn net.Conn, t time.Time) {
		_ = conn.SetDeadline(t)
	}(c.conn, time.Time{})

	_, _, err := c.ReadResponse(220)
	if err != nil {
		return c, err
	}

	if err = c.stopAfter(StageConnect); err != nil {
		return c, err
	}
	return c, nil
}

// Close closes the connection.
func (c *Client) Close() error {
	err := c.Text.Close()
	if err == nil {
		c.Message(HintInfo, "Connection closed with remote host.")
	}
	return err
}

func (c *Client) ReadResponse(expectCode int) (int, string, error) {
	code, message, err := c.Text.ReadResponse(expectCode)
	// c.Messagef(HintRecv, "%s\n", message)
	//if err != nil {
	//	c.Messagef(HintError, err.Error())
	//}
	return code, message, err
}

func (c *Client) setConn(conn net.Conn) {
	c.conn = conn
	var r = io.TeeReader(conn, &recvWriter{c: c})
	var w io.Writer = conn

	rwc := struct {
		io.Reader
		io.Writer
		io.Closer
	}{
		Reader: r,
		Writer: w,
		Closer: conn,
	}
	c.Text = textproto.NewConn(rwc)
	_, c.tls = conn.(*tls.Conn)
}

type recvWriter struct {
	c    *Client
	buff string
}

func (w *recvWriter) Write(b []byte) (int, error) {
	length := len(b)
	lines := bytes.SplitAfter(b, []byte("\n"))
	for _, line := range lines {
		if bytes.HasSuffix(line, []byte("\n")) {
			s := w.buff + string(line[:len(line)-1])
			w.buff = ""
			w.c.Message(HintRecv, strings.TrimSuffix(s, "\r"))
		} else {
			w.buff += string(line)
		}
	}
	return length, nil
}

// hello runs EHLO or HELO, if needed
func (c *Client) hello() error {
	if c.didHello {
		return c.helloError
	}
	c.didHello = true
	if !c.config.SendHelo {
		err := c.ehlo()
		if err == nil {
			return nil
		}
	}
	c.helloError = c.helo()
	return c.helloError
}

// helper to send a command
func (c *Client) cmd(expectCode int, stage Stage, format string, args ...interface{}) (int, string, error) {
	c.Messagef(HintSend, format, args...)

	c.conn.SetDeadline(time.Now().Add(c.config.Timeout))
	defer c.conn.SetDeadline(time.Time{})

	id, err := c.Text.Cmd(format, args...)
	if err != nil {
		return 0, "", err
	}
	if err = c.dropAfterSend(stage); err != nil {
		return 0, "", err
	}

	c.Text.StartResponse(id)
	code, msg, err := c.ReadResponse(expectCode)
	c.Text.EndResponse(id)
	if err != nil || stage == StageNone {
		return code, msg, err
	}
	if saErr := c.stopAfter(stage); saErr != nil {
		return 0, "", saErr
	}
	return code, msg, nil
}

// helo sends the HELO greeting to the server. It should be used only when the
// server does not support ehlo.
func (c *Client) helo() error {
	c.ext = nil
	_, _, err := c.cmd(250, StageNone, "HELO %s", c.config.Helo)
	return err
}

// ehlo sends the EHLO (extended hello) greeting to the server. It
// should be the preferred greeting for servers that support it.
func (c *Client) ehlo() error {
	var stage Stage
	if c.config.UseStartTLS {
		if c.tls {
			stage = StageHelo
		} else {
			stage = StageFirstHelo
		}
	} else {
		stage = StageHelo // FIXME(steve): need to handle helo vs first-helo for the non-starttls case
	}
	cmd := "EHLO"
	_, msg, err := c.cmd(250, stage, "%s %s", cmd, c.config.Helo)
	if err != nil {
		return err
	}
	ext := make(map[string]string)
	extList := strings.Split(msg, "\n")
	if len(extList) > 1 {
		extList = extList[1:]
		for _, line := range extList {
			args := strings.SplitN(line, " ", 2)
			if len(args) > 1 {
				ext[args[0]] = args[1]
			} else {
				ext[args[0]] = ""
			}
		}
	}
	if mechs, ok := ext["AUTH"]; ok {
		c.auth = strings.Split(mechs, " ")
	}
	c.ext = ext
	return err
}

// StartTLS sends the STARTTLS command and encrypts all further communication.
// Only servers that advertise the STARTTLS extension support this function.
//
// A nil config is equivalent to a zero tls.Config.
//
// If server returns an error, it will be of type *SMTPError.
func (c *Client) StartTLS(config *tls.Config) error {
	if err := c.hello(); err != nil {
		return err
	}
	_, _, err := c.cmd(220, StageStarttls, "STARTTLS")
	if err != nil {
		return err
	}
	if config == nil {
		config = &tls.Config{}
	}
	if config.ServerName == "" {
		// Make a copy to avoid polluting argument
		config = config.Clone()
		config.ServerName = c.remoteHost
	}
	c.setConn(tls.Client(c.conn, config))
	return c.ehlo()
}

// Mail issues a MAIL command to the server using the provided email address.
// If the server supports the 8BITMIME extension, Mail adds the BODY=8BITMIME
// parameter.
// This initiates a mail transaction and is followed by one or more Rcpt calls.
//
// If opts is not nil, MAIL arguments provided in the structure will be added
// to the command. Handling of unsupported options depends on the extension.
//
// If server returns an error, it will be of type *SMTPError.
func (c *Client) Mail(from string) error {
	if err := c.hello(); err != nil {
		return err
	}
	cmdStr := "MAIL FROM:<%s>"
	if _, ok := c.ext["8BITMIME"]; ok {
		cmdStr += " BODY=8BITMIME"
	}
	if _, ok := c.ext["SIZE"]; ok && c.config.Size != 0 {
		cmdStr += " SIZE=" + strconv.Itoa(c.config.Size)
	}
	// TODO(steve): read rfc8689
	//if opts != nil && opts.RequireTLS {
	//	if _, ok := c.ext["REQUIRETLS"]; ok {
	//		cmdStr += " REQUIRETLS"
	//	} else {
	//		return errors.New("smtp: server does not support REQUIRETLS")
	//	}
	//}

	if c.config.SmtpUTF8 {
		if _, ok := c.ext["SMTPUTF8"]; ok {
			cmdStr += " SMTPUTF8"
		} else {
			c.Message(HintError, "server does not support SMTPUTF8")
			return errors.New("smtp: server does not support SMTPUTF8")
		}
	}

	// TODO(steve): consider auth
	//if opts != nil && opts.Auth != nil {
	//	if _, ok := c.ext["AUTH"]; ok {
	//		cmdStr += " AUTH=" + encodeXtext(*opts.Auth)
	//	}
	//	// We can safely discard parameter if server does not support AUTH.
	//}
	_, _, err := c.cmd(250, StageMail, cmdStr, from)
	return err
}

// Rcpt issues a RCPT command to the server using the provided email address.
// A call to Rcpt must be preceded by a call to Mail and may be followed by
// a Data call or another Rcpt call.
//
// If server returns an error, it will be of type *SMTPError.
func (c *Client) Rcpt(to string) error {
	if _, _, err := c.cmd(25, StageRcpt, "RCPT TO:<%s>", to); err != nil {
		return err
	}
	c.rcpts = append(c.rcpts, to)
	return nil
}

type dataCloser struct {
	c *Client
	io.WriteCloser
}

func (d *dataCloser) Close() error {
	d.WriteCloser.Close()

	d.c.conn.SetDeadline(time.Now().Add(d.c.config.Timeout))
	defer d.c.conn.SetDeadline(time.Time{})

	_, _, err := d.c.Text.ReadResponse(250)
	return err
}

// Data issues a DATA command to the server and returns a writer that
// can be used to write the mail headers and body. The caller should
// close the writer before calling any more methods on c. A call to
// Data must be preceded by one or more calls to Rcpt.
//
// If server returns an error, it will be of type *SMTPError.
func (c *Client) Data() (io.WriteCloser, error) {
	_, _, err := c.cmd(354, StageData, "DATA")
	if err != nil {
		return nil, err
	}
	return &dataCloser{c, c.Text.DotWriter()}, nil
}

// Reset sends the RSET command to the server, aborting the current mail
// transaction.
func (c *Client) Reset() error {
	if err := c.hello(); err != nil {
		return err
	}
	if _, _, err := c.cmd(250, StageNone, "RSET"); err != nil {
		return err
	}
	c.rcpts = nil
	return nil
}

// Noop sends the NOOP command to the server. It does nothing but check
// that the connection to the server is okay.
func (c *Client) Noop() error {
	if err := c.hello(); err != nil {
		return err
	}
	_, _, err := c.cmd(250, StageNone, "NOOP")
	return err
}

// Quit sends the QUIT command and closes the connection to the server.
//
// If Quit fails the connection is not closed, Close should be used
// in this case.
func (c *Client) Quit() error {
	if err := c.hello(); err != nil {
		return err
	}
	_, _, err := c.cmd(221, StageNone, "QUIT")
	if err != nil {
		return err
	}
	err = c.Text.Close()
	return err
}

func (c *Client) drop() error {
	c.Message(HintInfo, "Dropping connection")
	err := c.Text.Close()
	if err != nil {
		return err
	}
	return ExitError{nil, ExitOk}
}

func (c *Client) dropAfterSend(stage Stage) error {
	if stage == StageNone {
		return nil
	}
	if c.config.DropAfterSend == stage {
		return c.drop()
	}
	return nil
}

func (c *Client) stopAfter(stage Stage) error {
	if stage == StageNone {
		return nil
	}
	if c.config.QuitAfter == stage {
		err := c.Quit()
		if err != nil {
			return err
		}
		return ExitError{nil, ExitOk}
	}
	if c.config.DropAfter == stage {
		return c.drop()
	}
	return nil
}
