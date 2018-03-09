package main

import (
	"flag"
	"fmt"
	"log"
	"os/exec"
	"time"
)

func csiCode(ctrl rune, num ...int) string {
	const CSI = "\033["

	switch len(num) {
	case 1:
		return fmt.Sprintf("%s%d%c", CSI, num[0], ctrl)
	case 2:
		return fmt.Sprintf("%s%d;%d%c", CSI, num[0], num[1], ctrl)
	}
	return ""
}

func runCmd(cmdstr []string, cmdout chan<- string, sleepSec int) {
	cmdlen := len(cmdstr)
	if cmdlen == 0 {
		log.Fatalf("Command not Found.")
	}
	sleepTime := time.Duration(sleepSec) * time.Second
	for {
		var out []byte
		var err error
		switch cmdlen {
		case 1:
			out, err = exec.Command(cmdstr[0]).Output()
		default:
			out, err = exec.Command(cmdstr[0], cmdstr[1:]...).Output()
		}
		if err != nil {
			log.Fatal(err)
		}
		cmdout <- string(out)
		time.Sleep(sleepTime)
	}
}

func printOut(cmdout <-chan string) {
	const (
		Before = 0
		After  = 1
		All    = 2
	)
	const (
		Up      = 'A'
		Down    = 'B'
		Right   = 'C'
		Left    = 'D'
		Below   = 'E'
		Above   = 'F'
		Begin   = 'G'
		Move    = 'H'
		Clear   = 'J'
		Delete  = 'K'
		Forward = 'S'
		Back    = 'T'
	)

	for {
		out := <-cmdout
		fmt.Print(csiCode(Clear, All))
		fmt.Print(csiCode(Move, 1, 1))
		fmt.Print(out)
	}
}

func main() {
	var sleepSec int

	flag.IntVar(&sleepSec, "s", 1, "sleep second")
	flag.Parse()

	if len(flag.Args()) == 0 {
		log.Fatalf("Command not Found.")
	}

	cmdout := make(chan string)
	go runCmd(flag.Args(), cmdout, sleepSec)
	printOut(cmdout)
}
