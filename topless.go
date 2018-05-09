package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
	"./ioctl"
)

const (
	StdinBuf = 128
	SleepSecDef = 1
	CountMaxDef = 3
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

const (
	CSI     = "\033["
	CtrlD   = "\004"
	CtrlU   = "\025"
	UpKey   = "\033[A"
	DownKey = "\033[B"
)

const (
	Normal  = "\x1B[0m"
	Red     = "\x1B[31m"
	Green   = "\x1B[32m"
	Yellow  = "\x1B[33m"
	Blue    = "\x1B[34m"
	Magenta = "\x1B[35m"
	Cyan    = "\x1B[36m"
	White   = "\x1B[37m"
)

type strArray struct {
	elem   []string
	len    int
	orgLen int
	count  []int
}

type optToCmd struct {
	sleepSec float64
	force    bool
}

type stdinToCmd struct {
	wait chan bool
	exit chan bool
}

type stdinToWrite struct {
	refresh chan bool
	head    chan int
}

func newStrArray(str string, delim string, height int) strArray {
	elem := strings.Split(str, delim)
	orgLen := len(elem)
	length := orgLen
	if length > height {
		length = height
	}
	return strArray{
		elem:   elem,
		len:    length,
		orgLen: orgLen,
		count: make([]int, orgLen),
	}
}

func csiCode(ctrl rune, num ...int) string {
	switch len(num) {
	case 1:
		return fmt.Sprintf("%s%d%c", CSI, num[0], ctrl)
	case 2:
		return fmt.Sprintf("%s%d;%d%c", CSI, num[0], num[1], ctrl)
	}
	return ""
}

func getStdin(stdinChan chan<- string) {
	var err error
	for {
		input := make([]byte, StdinBuf)
		n, err := os.Stdin.Read(input)
		if err != nil {
			break
		}
		if n > 0 {
			stdinChan <- string(input[:n])
		}
	}
	close(stdinChan)
	log.Fatal(err)
}

func treatStdin(stdinChan <-chan string, chanCmd stdinToCmd, chanWrite stdinToWrite) {
	var wait bool
	var exit bool
	for stdin := range stdinChan {
		switch stdin {
		case "q":
			exit = !exit
			chanCmd.exit <- exit
		case "w":
			wait = !wait
			chanCmd.wait <- wait
		case "r":
			chanWrite.refresh <- true
		case CtrlD:
			chanWrite.head <- +10
		case CtrlU:
			chanWrite.head <- -10
		case DownKey:
			chanWrite.head <- +1
		case UpKey:
			chanWrite.head <- -1
		}
	}
}

func makeExecArray(cmdArray [][]string, length int) ([]*exec.Cmd, []*io.PipeReader, []*io.PipeWriter) {
	var execArray []*exec.Cmd
	var readArray []*io.PipeReader
	var writeArray []*io.PipeWriter

	for i := 0; i < length; i++ {
		switch len(cmdArray[i]) {
		case 1:
			execArray = append(execArray, exec.Command(cmdArray[i][0]))
		default:
			execArray = append(execArray, exec.Command(cmdArray[i][0], cmdArray[i][1:]...))
		}
		if i != 0 {
			execArray[i].Stdin = readArray[i-1]
		}
		if i < length-1 {
			read, write := io.Pipe()
			readArray = append(readArray, read)
			writeArray = append(writeArray, write)
			execArray[i].Stdout = writeArray[i]
		}
	}
	return execArray, readArray, writeArray
}

func runCmdArray(cmdArray [][]string) (string, error) {
	var out bytes.Buffer
	var err error
	var i int
	length := len(cmdArray)

	execArray, _, writeArray := makeExecArray(cmdArray, length)
	execArray[length-1].Stdout = &out

	for i = 0; i < length; i++ {
		err = execArray[i].Start()
		if err != nil {
			return "", err
		}
	}
	for i = 0; i < length-1; i++ {
		err = execArray[i].Wait()
		writeArray[i].Close()
		if err != nil {
			return "", err
		}
	}
	err = execArray[length-1].Wait()
	return out.String(), err
}

func runCmdRepeatedly(cmdstr []string, cmdoutChan chan<- string, chanCmd stdinToCmd, optCmd optToCmd) error {
	var cmdout string
	var cmdArray [][]string
	var err error
	var wait bool
	var exit bool

	sleepTime := time.Duration(optCmd.sleepSec * 1000) * time.Millisecond
	cmdArray = append(cmdArray, cmdstr)
	for {
		select {
		case wait = <-chanCmd.wait:
		case exit = <-chanCmd.exit:
		default:
		}
		if exit {
			break
		}
		if wait {
			time.Sleep(sleepTime)
			continue
		}
		cmdout, err = runCmdArray(cmdArray)
		if !optCmd.force && err != nil {
			if cmdout != "" {
				fmt.Print(cmdout)
			}
			fmt.Println(err)
			break
		}
		cmdoutChan <- cmdout
		time.Sleep(sleepTime)
	}
	close(cmdoutChan)
	return err
}

func checkHead(line strArray, head int, dhead int, height int) int {
	if line.orgLen < height {
		return 0
	}
	head = head + dhead
	if head < 0 {
		head = 0
	} else if height+head > line.orgLen {
		head = line.orgLen - height
	}
	return head
}

func eraseToBegin(length int) {
	if length == 0 {
		return
	} else if length == 1 {
		fmt.Print(csiCode(Delete, All))
		fmt.Print(csiCode(Begin, 1))
	} else {
		for i := 0; i < length-1; i++ {
			fmt.Print(csiCode(Delete, All))
			fmt.Print(csiCode(Above, 1))
		}
	}
}

func moveToBegin(length int) {
	if length == 0 {
		return
	} else if length == 1 {
		fmt.Print(csiCode(Begin, 1))
	} else {
		fmt.Print(csiCode(Above, length-1))
	}
}

func wrapIn(width int, line string) string {
	if len(line) > width {
		return line[:width]
	} else {
		return line
	}
}

func coloring(color string, line string) string {
	return color + line + Normal
}

func printLine(line strArray, head int, width int) {
	last := line.len + head - 1
	for i := head; i < last; i++ {
		fmt.Print(csiCode(Delete, All))
		fmt.Println(wrapIn(width, line.elem[i]))
	}
	fmt.Print(csiCode(Delete, All))
	fmt.Print(wrapIn(width, line.elem[last]))
}

func checkLineCount(line strArray, i int) int {
	if line.count[i] > 0 {
		return line.count[i]-1
	} else {
		return line.count[i]
	}
}

func printLineDiff(old strArray, line strArray, head int, width int) []int {
	last := line.len + head - 1
	for i := head; i < last; i++ {
		if old.elem[i] == line.elem[i] {
			line.count[i] = checkLineCount(old, i)
			if line.count[i] == 1 {
				fmt.Print(csiCode(Delete, All))
				fmt.Println(wrapIn(width, line.elem[i]))
			} else {
				fmt.Print(csiCode(Below, 1))
			}
		} else {
			fmt.Print(csiCode(Delete, All))
			fmt.Println(coloring(Red, wrapIn(width, line.elem[i])))
			line.count[i] = CountMaxDef + 1
		}
	}
	if old.elem[last] == line.elem[last] {
		line.count[last] = checkLineCount(old, last)
		if line.count[last] == 1 {
			fmt.Print(csiCode(Delete, All))
			fmt.Println(wrapIn(width, line.elem[last]))
		} else {
			fmt.Print(csiCode(Below, 1))
		}
	} else {
		fmt.Print(csiCode(Delete, All))
		fmt.Print(coloring(Red, wrapIn(width, line.elem[last])))
		line.count[last] = CountMaxDef + 1
	}
	return line.count
}

func rewriteLines(cmdoutChan <-chan string, chanWrite stdinToWrite) {
	var oldline strArray
	var newline strArray
	var cmdout string
	var height, width int
	var head int

	for {
		winSize, err := ioctl.GetWinsize()
		if err != nil {
			log.Fatal(err)
		}
		height = int(winSize.Row) - 1
		width = int(winSize.Col)
		select {
		case refresh := <-chanWrite.refresh:
			if refresh {
				eraseToBegin(oldline.len)
				printLine(oldline, head, width)
			}
		case dhead := <-chanWrite.head:
			newhead := checkHead(oldline, head, dhead, height)
			if newhead != head {
				head = newhead
				eraseToBegin(oldline.len)
				printLine(oldline, head, width)
			}
		case cmdout = <-cmdoutChan:
			newline = newStrArray(cmdout, "\n", height)
			head = checkHead(newline, head, 0, height)
			if oldline.len != newline.len {
				eraseToBegin(oldline.len)
				printLine(newline, head, width)
			} else {
				moveToBegin(oldline.len)
				newline.count = printLineDiff(oldline, newline, head, width)
			}
			oldline = newline
		}
	}
}

func main() {
	var interactive bool
	var shell bool
	var optCmd optToCmd
	var err error
	var ret int

	flag.Usage = func() {
		fmt.Printf("Usage: %s [-s sec] [-i] [-sh] [-f] command\n\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Float64Var(&optCmd.sleepSec, "s", SleepSecDef, "sleep second")
	flag.BoolVar(&interactive, "i", false, "interactive")
	flag.BoolVar(&shell, "sh", false, "execute through the shell")
	flag.BoolVar(&optCmd.force, "f", false, "ignore execute error")
	flag.Parse()

	if len(flag.Args()) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	cmd := flag.Args()
	if shell {
		cmd = append([]string{"sh", "-c"}, strings.Join(cmd, " "))
	}

	chanCmd := stdinToCmd{make(chan bool), make(chan bool)}
	chanWrite := stdinToWrite{make(chan bool), make(chan int)}

	err = ioctl.SetOrgTermios()
	if err != nil {
		log.Fatal(err)
	}

	if !interactive {
		err = ioctl.ChangeTermiosLflag(^(ioctl.ECHO|ioctl.ICANNON))
		if err != nil {
			log.Fatal(err)
		}
		stdinChan := make(chan string)
		go treatStdin(stdinChan, chanCmd, chanWrite)
		go getStdin(stdinChan)
	}

	cmdoutChan := make(chan string)
	go rewriteLines(cmdoutChan, chanWrite)
	err = runCmdRepeatedly(cmd, cmdoutChan, chanCmd, optCmd)
	if err != nil {
		ret = 1
	}
	err = ioctl.ResetTermiosLflag()
	if err != nil {
		log.Fatal(err)
	}
	os.Exit(ret)
}
