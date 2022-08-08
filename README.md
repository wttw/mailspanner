# mailspanner
SMTP client tool.

`mailspanner` is a commandline client for sending email, in a way that's useful for diagnosing
delivery problems. It's intended to be cross-platform, with easy Windows support.

This is [not SWAKS](https://jetmore.org/john/code/swaks/), and doesn't intend to replace SWAKS,
but should feel similar enough that basic tutorials using a common subset of their functionality
should work with either.

```
Usage of mailspanner:
  -4, --4                        Force IPv4
  -6, --6                        Force IPv6
      --add-header stringArray   Add header
      --ah stringArray           Add header
      --body string              Specify the body of the email (default "This is a test mailing.")
      --copy-routing string      Choose destination server as though mail were sent to this domain
      --da string                Drop connection at this point
      --das string               Drop connection after sending response at this point
      --data string              Use the argument as the entire contents of DATA (default "Date: 
%DATE%\\nTo: %TO_ADDRESS%\\nFrom: %FROM_ADDRESS%\\nSubject: test %DATE%\\nMessage-Id: 
<%MESSAGEID%>\\nX-Mailer: mailspanner v%MAILSPANNER_VERSION% 
github.com/wttw/mailspanner\\n%NEW_HEADERS%\\n%BODY%\\n")
      --drop-after string        Drop connection at this point
      --drop-after-send string   Drop connection after sending response at this point
      --dump                     Dump configuration to stdout and exit
      --dump-mail                Dump the generated data to stdout and exit
      --ehlo string              Value to use for HELO
      --f string                 Envelope sender of email
  -f, --from string              Envelope sender of email
      --ha                       Hide all information sent to the terminal
      --header stringArray       Set header
      --helo string              Value to use for HELO
      --hi                       Hide informational messages
      --hide-all                 Hide all information sent to the terminal
      --hide-informational       Hide informational messages
      --hide-receive             Hide the responses received
      --hide-send                Hide the commands sent
      --hr                       Hide the responses received
      --hs                       Hide the commands sent
      --no-data-fixup            Don't clean up the data section
      --p string                 The port to connect to
      --pipeline                 Use ESMTP pipelining
  -p, --port string              The port to connect to
      --q string                 Quit after this point
      --quit string              Quit after this point
  -q, --quit-after string        Quit after this point
      --s string                 The server[:port] to connect to
  -s, --server string            The server[:port] to connect to
      --size int[=-1]            Send SIZE ESMTP option
      --smtputf8                 Request SMTPUTF8
      --suppress-data            Don't display the contents of data
      --t strings                Comma-separated list of recipient email addresses
      --timeout duration         Timeout after this long (default 30s)
      --timing                   Display timestamps
  -t, --to strings               Comma-separated list of recipient email addresses
```

