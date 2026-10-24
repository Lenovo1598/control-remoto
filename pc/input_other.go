//go:build !windows

package main

import "log"

// Modo simulado para probar fuera de Windows: solo registra las acciones.
type fakeInput struct{}

func newInput() Input {
	log.Println("(Modo simulado: no es Windows, las acciones solo se muestran)")
	return fakeInput{}
}

func (fakeInput) Move(dx, dy int32)          { log.Printf("[sim] mover %d,%d", dx, dy) }
func (fakeInput) Button(b string, down bool) { log.Printf("[sim] boton %s presionado=%v", b, down) }
func (fakeInput) Click(b string)             { log.Printf("[sim] clic %s", b) }
func (fakeInput) Scroll(a int32)             { log.Printf("[sim] scroll %d", a) }
func (fakeInput) Text(s string)              { log.Printf("[sim] texto %q", s) }
func (fakeInput) Key(k KeyDef)               { log.Printf("[sim] tecla vk=0x%02X", k.VK) }
