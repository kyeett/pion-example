package main

import (
	"fmt"

	"github.com/peterhellberg/gfx"
	"golang.org/x/image/colornames"

	"github.com/hajimehoshi/ebiten"
	"github.com/hajimehoshi/ebiten/ebitenutil"
)

type Game struct {
	player *Player
}

func (g *Game) Update(screen *ebiten.Image) error {
	fmt.Println("Update called")
	g.player.UpdatePosition()
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	fmt.Println("Draw called")
	gfx.DrawCicleFast(screen, g.player.Position, g.player.Size, colornames.Red)

	ebitenutil.DebugPrint(screen,
		fmt.Sprintf("Move the red point by mouse wheel\n(%0.2f, %0.2f)", g.player.Position.X, g.player.Position.Y))

}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return screenWidth, screenHeight
}
