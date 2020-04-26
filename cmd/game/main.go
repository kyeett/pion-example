package main

import (
	"log"

	"github.com/hajimehoshi/ebiten"
	"github.com/peterhellberg/gfx"
)

const (
	screenWidth  = 320
	screenHeight = 240
)

type Player struct {
	Position gfx.Vec
	Size     float64
}

func (p *Player) UpdatePosition() {
	_, dy := ebiten.Wheel()
	p.Size = gfx.Clamp(p.Size+dy, 5, 30)

	x, y := ebiten.CursorPosition()
	p.Position.X = gfx.Clamp(float64(x), p.Size, screenWidth-p.Size)
	p.Position.Y = gfx.Clamp(float64(y), p.Size, screenHeight-p.Size)
}

func main() {
	g := Game{
		player: &Player{
			Position: gfx.V(screenWidth/2, screenHeight/2),
			Size:     20,
		},
	}
	if err := ebiten.RunGame(&g); err != nil {
		log.Fatal(err)
	}
}
