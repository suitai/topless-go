package main

import (
	"flag"
	"fmt"
	"golang.org/x/sys/unix"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

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
	for {
		out := <-cmdout
		fmt.Print(csiCode(Clear, All))
		fmt.Print(csiCode(Move, 1, 1))
		fmt.Print(out)
	}
}

func cutExtraLines(oldlinenum int, newlinenum int, height int) {
	if oldlinenum > height {
		oldlinenum = height
	}
	if newlinenum > height {
		newlinenum = height
	}
	if oldlinenum > newlinenum {
		for i := 0; i < oldlinenum-newlinenum; i++ {
			fmt.Print(csiCode(Delete, All))
			fmt.Print(csiCode(Above, 1))
		}
	}
}

func moveToBegin(oldlinenum int, newlinenum int, height int) {
	linenum := oldlinenum

	if oldlinenum > newlinenum {
		linenum = newlinenum
	}
	if linenum > height {
		linenum = height
	}

	if linenum == 0 {
		return
	} else if linenum == 1 {
		fmt.Print(csiCode(Begin, 1))
	} else {
		fmt.Print(csiCode(Above, linenum-1))
	}
}

func printLineDiff(oldlines []string, oldlinenum int, newlines []string, newlinenum int, height int) {
	linenum := newlinenum

	if linenum > height {
		linenum = height
	}
	for i := 0; i < linenum; i++ {
		if i < oldlinenum && newlines[i] != "" && oldlines[i] == newlines[i] {
			fmt.Print(csiCode(Below, 1))
			continue
		}
		fmt.Print(csiCode(Delete, All))
		if i < linenum-1 {
			fmt.Println(newlines[i])
		} else {
			fmt.Print(newlines[i])
		}
	}
}

func getWinHeight() int {
	size, err := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ)
	if err != nil {
		log.Fatal(err)
	}
	return int(size.Row)
}

func rewriteLines(cmdout <-chan string) {
	var oldlines []string
	oldlinenum := 0
	for {
		out := <-cmdout
		height := getWinHeight()
		lines := strings.Split(out, "\n")
		linenum := len(lines)
		cutExtraLines(oldlinenum, linenum, height)
		moveToBegin(oldlinenum, linenum, height)
		printLineDiff(oldlines, oldlinenum, lines, linenum, height)
		oldlines = lines
		oldlinenum = linenum
	}
}

func main() {
	var sleepSec int
	var interactive bool

	flag.IntVar(&sleepSec, "s", 1, "sleep second")
	flag.BoolVar(&interactive, "i", false, "interactive")

	flag.Parse()

	if len(flag.Args()) == 0 {
		log.Fatalf("Command not Found.")
	}
	if !interactive {
		exec.Command("stty", "-F", "/dev/tty", "cbreak", "min", "1").Run()
		exec.Command("stty", "-F", "/dev/tty", "-echo").Run()
		defer exec.Command("stty", "-F", "/dev/tty", "echo").Run()
	}

	cmdout := make(chan string)
	go runCmd(flag.Args(), cmdout, sleepSec)
	rewriteLines(cmdout)
}
