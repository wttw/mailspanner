package main

import (
	"errors"
	"fmt"
	"math/rand"
	"net/textproto"
	"os"
	"time"
)

const AppName = "mailspanner"

func main() {
	rand.Seed(time.Now().UnixNano())
	var c Config
	err := c.ParseFlags(os.Args[1:])
	if err != nil {
		Fatal(err)
	}

	payload, err := MakePayload(c)
	if err != nil {
		Fatal(err)
	}
	if c.dumpMail {
		fmt.Println(payload)
		Exit(ExitOk)
	}
	err = send(c, payload)
	if err != nil {
		var tpErr *textproto.Error
		if !errors.As(err, &tpErr) {
			Fatal(err)
		}
	}
	Exit(ExitOk)
}
