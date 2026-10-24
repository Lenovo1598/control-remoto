//go:build windows

package main

import (
	"log"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

var (
	user32        = syscall.NewLazyDLL("user32.dll")
	procSendInput = user32.NewProc("SendInput")
)

const (
	inputMouse    = 0
	inputKeyboard = 1

	mMove      = 0x0001
	mLeftDown  = 0x0002
	mLeftUp    = 0x0004
	mRightDown = 0x0008
	mRightUp   = 0x0010
	mMidDown   = 0x0020
	mMidUp     = 0x0040
	mWheel     = 0x0800

	kExtended = 0x0001
	kUp       = 0x0002
	kUnicode  = 0x0004
)

// Estructuras INPUT de Windows (64 bits: 40 bytes cada una).
type mouseInput struct {
	Type      uint32
	_         uint32
	Dx, Dy    int32
	MouseData uint32
	Flags     uint32
	Time      uint32
	_         uint32
	Extra     uintptr
}

type keybdInput struct {
	Type  uint32
	_     uint32
	Vk    uint16
	Scan  uint16
	Flags uint32
	Time  uint32
	_     uint32
	Extra uintptr
	_     [8]byte
}

func init() {
	if unsafe.Sizeof(mouseInput{}) != 40 || unsafe.Sizeof(keybdInput{}) != 40 {
		log.Fatal("Esta versión es solo para Windows de 64 bits")
	}
}

type winInput struct{}

func newInput() Input { return winInput{} }

func sendMouse(in ...mouseInput) {
	if len(in) > 0 {
		procSendInput.Call(uintptr(len(in)), uintptr(unsafe.Pointer(&in[0])), unsafe.Sizeof(in[0]))
	}
}

func sendKeys(in ...keybdInput) {
	if len(in) > 0 {
		procSendInput.Call(uintptr(len(in)), uintptr(unsafe.Pointer(&in[0])), unsafe.Sizeof(in[0]))
	}
}

func mouse(flags uint32) mouseInput { return mouseInput{Type: inputMouse, Flags: flags} }

func buttonFlags(b string) (down, up uint32) {
	switch b {
	case "r":
		return mRightDown, mRightUp
	case "m":
		return mMidDown, mMidUp
	default:
		return mLeftDown, mLeftUp
	}
}

func (winInput) Move(dx, dy int32) {
	m := mouse(mMove)
	m.Dx, m.Dy = dx, dy
	sendMouse(m)
}

func (winInput) Button(b string, down bool) {
	d, u := buttonFlags(b)
	if down {
		sendMouse(mouse(d))
	} else {
		sendMouse(mouse(u))
	}
}

func (winInput) Click(b string) {
	d, u := buttonFlags(b)
	sendMouse(mouse(d), mouse(u))
}

func (winInput) Scroll(amount int32) {
	m := mouse(mWheel)
	m.MouseData = uint32(amount)
	sendMouse(m)
}

func (winInput) Text(s string) {
	units := utf16.Encode([]rune(s))
	ev := make([]keybdInput, 0, len(units)*2)
	for _, u := range units {
		ev = append(ev,
			keybdInput{Type: inputKeyboard, Scan: u, Flags: kUnicode},
			keybdInput{Type: inputKeyboard, Scan: u, Flags: kUnicode | kUp})
	}
	sendKeys(ev...)
}

func (winInput) Key(k KeyDef) {
	var f uint32
	if k.Ext {
		f = kExtended
	}
	sendKeys(
		keybdInput{Type: inputKeyboard, Vk: k.VK, Flags: f},
		keybdInput{Type: inputKeyboard, Vk: k.VK, Flags: f | kUp})
}
