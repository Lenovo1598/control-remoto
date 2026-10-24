// Control Remoto — programa de Windows.
// Recibe órdenes del celular (Wi-Fi o Tailscale) y mueve el mouse, hace clics,
// escribe texto y controla multimedia/presentaciones.
package main

import (
	"bufio"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	_ "embed"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const version = "1.0"

//go:embed web/index.html
var clientHTML []byte

// Input es lo que el programa sabe hacer en la PC.
type Input interface {
	Move(dx, dy int32)
	Button(b string, down bool)
	Click(b string)
	Scroll(amount int32)
	Text(s string)
	Key(k KeyDef)
}

type KeyDef struct {
	VK  uint16
	Ext bool
}

// Teclas permitidas desde el celular.
var keys = map[string]KeyDef{
	"enter": {0x0D, false}, "backspace": {0x08, false}, "esc": {0x1B, false},
	"tab": {0x09, false}, "space": {0x20, false}, "f5": {0x74, false},
	"left": {0x25, true}, "up": {0x26, true}, "right": {0x27, true}, "down": {0x28, true},
	"pgup": {0x21, true}, "pgdn": {0x22, true}, "home": {0x24, true}, "end": {0x23, true},
	"delete": {0x2E, true},
	"volup":  {0xAF, false}, "voldown": {0xAE, false}, "mute": {0xAD, false},
	"playpause": {0xB3, false}, "next": {0xB0, false}, "prev": {0xB1, false}, "stop": {0xB2, false},
}

// ---------------------------------------------------------------- configuración

type Config struct {
	Port int    `json:"port"`
	Code string `json:"code"`
}

func configPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	return filepath.Join(dir, "ControlRemoto", "config.json")
}

func randomCode() string {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		log.Fatal(err)
	}
	return fmt.Sprintf("%06d", n.Int64())
}

func loadConfig(newCode bool, port int) Config {
	c := Config{}
	if b, err := os.ReadFile(configPath()); err == nil {
		_ = json.Unmarshal(b, &c)
	}
	changed := false
	if port > 0 && port != c.Port {
		c.Port, changed = port, true
	}
	if c.Port == 0 {
		c.Port, changed = 8765, true
	}
	if newCode || len(c.Code) != 6 {
		c.Code, changed = randomCode(), true
	}
	if changed {
		_ = os.MkdirAll(filepath.Dir(configPath()), 0o700)
		b, _ := json.MarshalIndent(c, "", "  ")
		_ = os.WriteFile(configPath(), b, 0o600)
	}
	return c
}

// ---------------------------------------------------------------- protección contra intentos

type limiter struct {
	mu    sync.Mutex
	fails map[string]int
	until map[string]time.Time
}

func (l *limiter) blocked(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return time.Now().Before(l.until[ip])
}

func (l *limiter) fail(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.fails[ip]++
	if l.fails[ip] >= 5 {
		l.until[ip] = time.Now().Add(time.Minute)
		l.fails[ip] = 0
	}
}

func (l *limiter) ok(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.fails, ip)
}

// ---------------------------------------------------------------- servidor

type server struct {
	cfg   Config
	input Input
	lim   *limiter
}

func hostOf(addr string) string {
	h, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return h
}

func (s *server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(clientHTML)
}

func (s *server) handleWS(w http.ResponseWriter, r *http.Request) {
	ip := hostOf(r.RemoteAddr)
	if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		http.Error(w, "se esperaba websocket", http.StatusBadRequest)
		return
	}
	if s.lim.blocked(ip) {
		http.Error(w, "demasiados intentos", http.StatusTooManyRequests)
		return
	}
	if subtle.ConstantTimeCompare([]byte(r.URL.Query().Get("t")), []byte(s.cfg.Code)) != 1 {
		s.lim.fail(ip)
		log.Printf("Código incorrecto desde %s", ip)
		http.Error(w, "codigo invalido", http.StatusForbidden)
		return
	}
	s.lim.ok(ip)

	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "no soportado", http.StatusInternalServerError)
		return
	}
	conn, rw, err := hj.Hijack()
	if err != nil {
		return
	}
	defer conn.Close()
	if tc, ok := conn.(*net.TCPConn); ok {
		_ = tc.SetNoDelay(true)
	}
	sum := sha1.Sum([]byte(r.Header.Get("Sec-WebSocket-Key") + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	_, _ = rw.WriteString("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + base64.StdEncoding.EncodeToString(sum[:]) + "\r\n\r\n")
	if rw.Flush() != nil {
		return
	}

	log.Printf("Celular conectado: %s", ip)
	held := map[string]bool{}
	defer func() {
		for b := range held {
			s.input.Button(b, false)
		}
		log.Printf("Celular desconectado: %s", ip)
	}()
	s.readLoop(conn, rw.Reader, held)
}

func wsFrame(opcode byte, payload []byte) []byte {
	n := len(payload)
	var h []byte
	switch {
	case n < 126:
		h = []byte{0x80 | opcode, byte(n)}
	case n < 65536:
		h = []byte{0x80 | opcode, 126, byte(n >> 8), byte(n)}
	default:
		h = make([]byte, 10)
		h[0], h[1] = 0x80|opcode, 127
		binary.BigEndian.PutUint64(h[2:], uint64(n))
	}
	return append(h, payload...)
}

func (s *server) readLoop(conn net.Conn, r *bufio.Reader, held map[string]bool) {
	hdr := make([]byte, 8)
	for {
		// El celular manda una señal cada 15 s; si pasa más de 1 minuto, se corta.
		_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		if _, err := io.ReadFull(r, hdr[:2]); err != nil {
			return
		}
		opcode := hdr[0] & 0x0F
		masked := hdr[1]&0x80 != 0
		n := uint64(hdr[1] & 0x7F)
		switch n {
		case 126:
			if _, err := io.ReadFull(r, hdr[:2]); err != nil {
				return
			}
			n = uint64(binary.BigEndian.Uint16(hdr[:2]))
		case 127:
			if _, err := io.ReadFull(r, hdr[:8]); err != nil {
				return
			}
			n = binary.BigEndian.Uint64(hdr[:8])
		}
		if n > 64*1024 {
			return
		}
		var mask [4]byte
		if masked {
			if _, err := io.ReadFull(r, mask[:]); err != nil {
				return
			}
		}
		payload := make([]byte, n)
		if _, err := io.ReadFull(r, payload); err != nil {
			return
		}
		for i := range payload {
			payload[i] ^= mask[i&3]
		}
		switch opcode {
		case 0x8: // cerrar
			_, _ = conn.Write(wsFrame(0x8, nil))
			return
		case 0x9: // ping
			_, _ = conn.Write(wsFrame(0xA, payload))
		case 0x1: // texto
			s.dispatch(payload, held)
		}
	}
}

type message struct {
	T string  `json:"t"`
	X float64 `json:"x"`
	Y float64 `json:"y"`
	B string  `json:"b"`
	S string  `json:"s"`
	K string  `json:"k"`
}

func clamp(v, lim float64) int32 {
	if math.IsNaN(v) {
		return 0
	}
	return int32(math.Max(-lim, math.Min(lim, v)))
}

func (s *server) dispatch(raw []byte, held map[string]bool) {
	var m message
	if json.Unmarshal(raw, &m) != nil {
		return
	}
	b := m.B
	if b != "r" && b != "m" {
		b = "l"
	}
	switch m.T {
	case "m":
		s.input.Move(clamp(m.X, 1000), clamp(m.Y, 1000))
	case "c":
		s.input.Click(b)
	case "d":
		s.input.Button(b, true)
		held[b] = true
	case "u":
		s.input.Button(b, false)
		delete(held, b)
	case "s":
		s.input.Scroll(clamp(m.Y, 2400))
	case "k":
		txt := []rune(m.S)
		if len(txt) > 1000 {
			txt = txt[:1000]
		}
		s.input.Text(string(txt))
	case "key":
		if k, ok := keys[m.K]; ok {
			s.input.Key(k)
		}
	}
}

// ---------------------------------------------------------------- direcciones

func addresses() (lan, tailscale []string) {
	_, tsNet, _ := net.ParseCIDR("100.64.0.0/10")
	ifaces, _ := net.Interfaces()
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := ifc.Addrs()
		for _, a := range addrs {
			ipn, ok := a.(*net.IPNet)
			if !ok || ipn.IP.To4() == nil {
				continue
			}
			ip := ipn.IP.To4()
			switch {
			case tsNet.Contains(ip) || strings.Contains(strings.ToLower(ifc.Name), "tailscale"):
				tailscale = append(tailscale, ip.String())
			case ip.IsPrivate():
				lan = append(lan, ip.String())
			}
		}
	}
	return
}

func waitAndExit(code int) {
	fmt.Println("\nPresioná Enter para cerrar...")
	_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
	os.Exit(code)
}

func main() {
	newCode := flag.Bool("nuevo-codigo", false, "genera un código de vinculación nuevo")
	port := flag.Int("puerto", 0, "puerto a usar (por defecto 8765)")
	flag.Parse()
	log.SetFlags(log.Ltime)

	cfg := loadConfig(*newCode, *port)
	s := &server{cfg: cfg, input: newInput(),
		lim: &limiter{fails: map[string]int{}, until: map[string]time.Time{}}}

	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Port))
	if err != nil {
		fmt.Printf("No se pudo abrir el puerto %d: %v\n", cfg.Port, err)
		fmt.Println("¿Ya hay otra ventana de Control Remoto abierta?")
		waitAndExit(1)
	}

	lan, ts := addresses()
	line := strings.Repeat("─", 58)
	fmt.Println(line)
	fmt.Printf("  CONTROL REMOTO %s — listo\n", version)
	fmt.Println(line)
	fmt.Printf("  Código de vinculación:   %s\n\n", cfg.Code)
	if len(lan) > 0 {
		fmt.Printf("  IP Wi-Fi (en casa):      %s\n", strings.Join(lan, "  ·  "))
	} else {
		fmt.Println("  IP Wi-Fi (en casa):      no detectada")
	}
	if len(ts) > 0 {
		fmt.Printf("  IP Tailscale (internet): %s\n", strings.Join(ts, "  ·  "))
	} else {
		fmt.Println("  IP Tailscale (internet): no detectada (¿Tailscale está abierto?)")
	}
	fmt.Printf("  Puerto:                  %d\n", cfg.Port)
	if len(lan) > 0 {
		fmt.Printf("\n  También desde el navegador del celular:\n  http://%s:%d/?c=%s\n", lan[0], cfg.Port, cfg.Code)
	}
	fmt.Println(line)
	fmt.Println("  Dejá esta ventana abierta mientras lo uses.")
	fmt.Println(line)

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/ws", s.handleWS)
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	if err := srv.Serve(ln); err != nil {
		fmt.Println("Error:", err)
		waitAndExit(1)
	}
}
