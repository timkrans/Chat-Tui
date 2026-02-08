package main

import (
	"fmt"
	"os"
	"runtime"
	"syscall"
	"unsafe"
)

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
	if runtime.GOOS == "darwin" {
		fmt.Print("\033c")
	} else {
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

type ListView struct {
	items   []string
	index   int
	offset  int
	height  int
	width   int
	scrollY int
}

func (l *ListView) draw() {
	for i := 0; i < l.height; i++ {
		move(l.width-3, 3+i)
		pos := l.offset + i
		if pos >= len(l.items) {
			fmt.Print("~")
			continue
		}
		if pos == l.index {
			fmt.Print("> ", l.items[pos])
		} else {
			fmt.Print("  ", l.items[pos])
		}
	}
}

func (l *ListView) handle(b byte) {
	if b == '\033' {
		b2 := readByte()
		if b2 == '[' {
			b3 := readByte()
			if b3 == 'A' && l.index > 0 {
				l.index--
				if l.index < l.offset {
					l.offset--
				}
			} else if b3 == 'B' && l.index < len(l.items)-1 {
				l.index++
				if l.index >= l.offset+l.height {
					l.offset++
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
	fmt.Print("Input: ")
	fmt.Print(string(i.text))
}

func (i *InputView) handle(b byte) {
	if b == 127 || b == 8 {
		if len(i.text) > 0 {
			i.text = i.text[:len(i.text)-1]
		}
		return
	}
	if b == '\r' {
		i.text = nil
		return
	}
	if b >= 32 && b <= 126 {
		i.text = append(i.text, rune(b))
	}
}

func main() {
	if runtime.GOOS == "windows" {
		fmt.Println("windows not supported")
		return
	}

	old := makeRaw()
	defer restore(old)
	hideCursor()
	defer showCursor()

	list := ListView{
		items: []string{
			"Alpha", "Bravo", "Charlie", "Delta", "Echo",
			"Foxtrot", "Golf", "Hotel", "India", "Juliet",
			"Kilo", "Lima", "Mike", "November", "Oscar",
		},
		height: 8,
		width:  40,
	}

	input := InputView{}

	for {
		clear()
		move(2, 1)

		fmt.Print("List view (Up/Down scroll, q quit)\n")
		list.draw()

		move(2, 15)
		fmt.Print("Input: ")
		input.draw()

		b := readByte()
		if b == 'q' {
			break
		}

		list.handle(b)
		input.handle(b)
	}
}
