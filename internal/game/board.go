package game

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// Layout is the fixed Words With Friends premium square layout.
type Layout [BoardSize][BoardSize]Bonus

// Words With Friends board:
// T = triple word, D = double word, t = triple letter,
// d = double letter, s = start square.
var wwfRows = [BoardSize]string{
	"...T..t.t..T...",
	"..d..D...D..d..",
	".d..d.....d..d.",
	"T..t...D...t..T",
	"..d...d.d...d..",
	".D...t...t...D.",
	"t...d.....d...t",
	"...D...s...D...",
	"t...d.....d...t",
	".D...t...t...D.",
	"..d...d.d...d..",
	"T..t...D...t..T",
	".d..d.....d..d.",
	"..d..D...D..d..",
	"...T..t.t..T...",
}

func WWFLayout() Layout {
	var layout Layout
	for y, row := range wwfRows {
		for x, char := range row {
			switch char {
			case 'T':
				layout[y][x] = BonusTW
			case 'D':
				layout[y][x] = BonusDW
			case 't':
				layout[y][x] = BonusTL
			case 'd':
				layout[y][x] = BonusDL
			case 's':
				layout[y][x] = BonusStart
			default:
				layout[y][x] = BonusNone
			}
		}
	}
	return layout
}

func (layout Layout) At(coord Coord) Bonus {
	if !coord.Valid() {
		return BonusNone
	}
	return layout[coord.Y][coord.X]
}

func CenterCoord() Coord {
	return Coord{X: BoardSize / 2, Y: BoardSize / 2}
}

func BoardMap(tiles []PlacedTile) map[Coord]PlacedTile {
	result := make(map[Coord]PlacedTile, len(tiles))
	for _, tile := range tiles {
		result[tile.Coord()] = tile
	}
	return result
}

func SortedBoard(tiles []PlacedTile) []PlacedTile {
	result := append([]PlacedTile(nil), tiles...)
	sort.Slice(result, func(i, j int) bool {
		if result[i].Y == result[j].Y {
			return result[i].X < result[j].X
		}
		return result[i].Y < result[j].Y
	})
	return result
}

func BoardHash(tiles []PlacedTile) string {
	sorted := SortedBoard(tiles)
	builder := strings.Builder{}
	for _, placed := range sorted {
		fmt.Fprintf(&builder, "%d,%d,%s,%d,%t;", placed.X, placed.Y, placed.Tile.Letter, placed.Tile.Value, placed.Tile.Blank)
	}
	sum := sha256.Sum256([]byte(builder.String()))
	return hex.EncodeToString(sum[:])
}

func StateHash(game *Game) string {
	builder := strings.Builder{}
	fmt.Fprintf(&builder, "%s|%d|%s|", game.ID, game.Revision, game.Status)
	for _, player := range game.Players {
		fmt.Fprintf(&builder, "%s:%d;", player.ID, player.Score)
	}
	builder.WriteString(BoardHash(game.Board))
	builder.WriteString("|")
	for _, tile := range game.Bag {
		builder.WriteString(string(tile.ID))
		builder.WriteString(",")
	}
	sum := sha256.Sum256([]byte(builder.String()))
	return hex.EncodeToString(sum[:])
}

func RunLength(board map[Coord]PlacedTile, start Coord, direction Direction) (string, int) {
	deltaX, deltaY := 0, 0
	if direction == DirectionHorizontal {
		deltaX = 1
	} else {
		deltaY = 1
	}
	builder := strings.Builder{}
	length := 0
	coord := start
	for {
		placed, ok := board[coord]
		if !ok {
			break
		}
		builder.WriteString(placed.Tile.Letter)
		length++
		coord = Coord{X: coord.X + deltaX, Y: coord.Y + deltaY}
	}
	return builder.String(), length
}

func WordStart(board map[Coord]PlacedTile, coord Coord, direction Direction) Coord {
	deltaX, deltaY := 0, 0
	if direction == DirectionHorizontal {
		deltaX = -1
	} else {
		deltaY = -1
	}
	start := coord
	for {
		next := Coord{X: start.X + deltaX, Y: start.Y + deltaY}
		if _, ok := board[next]; !ok {
			return start
		}
		start = next
	}
}
