package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"unsafe"
)

type OllamaResponse struct {
	Response string `json:"response"`
}

type termState syscall.Termios

func ioctlGet() uintptr {
	if runtime.GOOS == "darwin" {
		return 0x40487413
	}
	return 0x5401
}

func ioctlSet() uintptr {
	if runtime.GOOS == "darwin" {
		return 0x80487414
	}
	return 0x5402
}

func makeRaw() *termState {
	fd := int(os.Stdin.Fd())
	var old syscall.Termios
	syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), ioctlGet(), uintptr(unsafe.Pointer(&old)))
	raw := old
	raw.Lflag &^= syscall.ICANON | syscall.ECHO
	raw.Iflag &^= syscall.ICRNL | syscall.IXON
	raw.Cc[syscall.VMIN] = 1
	raw.Cc[syscall.VTIME] = 0
	syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), ioctlSet(), uintptr(unsafe.Pointer(&raw)))
	s := termState(old)
	return &s
}

func restore(s *termState) {
	fd := int(os.Stdin.Fd())
	syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), ioctlSet(), uintptr(unsafe.Pointer((*syscall.Termios)(s))))
}

func clear() {
	if runtime.GOOS == "windows" {
		cmd := exec.Command("cmd", "/C", "cls")
		cmd.Stdout = os.Stdout
		cmd.Run()
	} else {
		if runtime.GOOS == "darwin"{
			fmt.Print("\033c")
		}
		fmt.Print("\033[H\033[2J")
	}
}

func move(x, y int) {
	fmt.Printf("\033[%d;%dH", y, x)
}

func hideCursor() {
	fmt.Print("\033[?25l")
}

func showCursor() {
	fmt.Print("\033[?25h")
}

func readByte() byte {
	var b [1]byte
	os.Stdin.Read(b[:])
	return b[0]
}

func termSize() (cols, rows int) {
	ws := &struct {
		row, col, x, y uint16
	}{}
	syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(os.Stdout.Fd()),
		uintptr(syscall.TIOCGWINSZ),
		uintptr(unsafe.Pointer(ws)),
	)
	return int(ws.col), int(ws.row)
}

func clampLine(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
}

type ListView struct {
	items  []string
	index  int
	offset int
	height int
}

func (l *ListView) draw() {
	for i := 0; i < l.height; i++ {
		move(2, 3+i)
		pos := l.offset + i
		if pos >= len(l.items) {
			fmt.Print("~")
			continue
		}
		line := clampLine(l.items[pos], 90)
		if pos == l.index {
			fmt.Print("> ", line)
		} else {
			fmt.Print("  ", line)
		}
	}
}

func (l *ListView) handle(b byte) {
	if b == '\033' {
		if readByte() == '[' {
			switch readByte() {
			case 'A':
				if l.index > 0 {
					l.index--
					if l.index < l.offset {
						l.offset--
					}
				}
			case 'B':
				if l.index < len(l.items)-1 {
					l.index++
					if l.index >= l.offset+l.height {
						l.offset++
					}
				}
			}
		}
	}
}

type InputView struct {
	text []rune
}

func (i *InputView) draw() {
	move(2, 15)
	fmt.Print("Input: ", string(i.text))
}

func (i *InputView) handle(b byte) bool {
	if b == 127 || b == 8 {
		if len(i.text) > 0 {
			i.text = i.text[:len(i.text)-1]
		}
		return false
	}
	if b == '\r' {
		return true
	}
	if b >= 32 && b <= 126 {
		i.text = append(i.text, rune(b))
	}
	return false
}

func getOllamaResponse(input string) string {
	url := "http://localhost:11434/api/generate"
	payload := fmt.Sprintf(`{"model":"llama3.2","prompt":%q,"stream":false}`, input)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer([]byte(payload)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	var r OllamaResponse
	json.Unmarshal(body, &r)
	respText := strings.ReplaceAll(r.Response, "\\n", "\n")
	respText = strings.ReplaceAll(r.Response, "\\t", "\t")
	return respText
}

func fullScreenView(text string) {
	clear()
	_, rows := termSize()
	lines := strings.Split(text, "\n")
	max := rows - 2
	for i := 0; i < len(lines) && i < max; i++ {
		move(2, 1+i)
		fmt.Print(lines[i])
	}
	move(2, rows)
	fmt.Print("Press any key to return")
	readByte()
}

func main() {
	old := makeRaw()
	defer restore(old)
	hideCursor()
	defer showCursor()

	list := ListView{height: 8}
	input := InputView{}

	for {
		clear()
		move(2, 1)
		fmt.Print("List view (Up/Down scroll, Enter open, q quit)")
		list.draw()
		input.draw()

		b := readByte()
		if b == 'q' {
			break
		}

		if b == '\r' && len(input.text) == 0 && list.index < len(list.items) {
			fullScreenView(list.items[list.index])
			continue
		}

		list.handle(b)
		submit := input.handle(b)

		if submit && len(input.text) > 0 {
			resp := getOllamaResponse(string(input.text))
			list.items = append(list.items, resp)
			input.text = nil
		}
	}
}
