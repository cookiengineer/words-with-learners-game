package server

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/cookiengineer/words-with-learners-app/internal/dictionary"
	"github.com/cookiengineer/words-with-learners-app/internal/game"
	"github.com/cookiengineer/words-with-learners-app/internal/view"
)

func findHint(value *game.Game, dict *dictionary.Dictionary) view.Hint {
	return findHintFor(value, dict, value.Current, false)
}

func findHintFor(value *game.Game, dict *dictionary.Dictionary, playerIndex int, firstOnly bool) view.Hint {
	if value == nil || dict == nil || playerIndex < 0 || playerIndex >= len(value.Players) {
		return view.Hint{Found: false}
	}
	player := &value.Players[playerIndex]
	best := view.Hint{Found: false}
	board := game.BoardMap(value.Board)
	for _, entry := range dict.Entries() {
		length := utf8.RuneCountInString(entry.Word)
		if length < 2 || length > value.Config.MaxWordLength {
			continue
		}
		for _, direction := range []game.Direction{game.DirectionHorizontal, game.DirectionVertical} {
			for startY := 0; startY < game.BoardSize; startY++ {
				for startX := 0; startX < game.BoardSize; startX++ {
					start := game.Coord{X: startX, Y: startY}
					if direction == game.DirectionHorizontal && startX+length > game.BoardSize {
						continue
					}
					if direction == game.DirectionVertical && startY+length > game.BoardSize {
						continue
					}
					placements, ok := matchWord(player.Rack, board, entry.Word, start, direction)
					if !ok || len(placements) == 0 {
						continue
					}
					_, words, score, err := game.ValidatePlacement(value, dict, placements)
					if err != nil {
						continue
					}
					bingo := len(placements) == len(player.Rack)
					if bingo {
						score += value.Config.BingoBonus
					}
					if !best.Found || score > best.Score {
						hint := strings.TrimSpace(entry.Hint)
						if hint == "" {
							hint = "a word with " + pluralLetters(length)
						}
						best = view.Hint{Found: true, Hint: hint, Length: length, Score: score}
						if firstOnly {
							return best
						}
					}
					_ = words
				}
			}
		}
	}
	return best
}

func matchWord(rack []game.Tile, board map[game.Coord]game.PlacedTile, word string, start game.Coord, direction game.Direction) ([]game.Placement, bool) {
	available := append([]game.Tile(nil), rack...)
	used := make([]bool, len(available))
	placements := make([]game.Placement, 0, len(word))
	deltaX, deltaY := 0, 0
	if direction == game.DirectionHorizontal {
		deltaX = 1
	} else {
		deltaY = 1
	}
	coord := start
	for _, letter := range word {
		if !coord.Valid() {
			return nil, false
		}
		wanted := string(letter)
		if existing, ok := board[coord]; ok {
			if existing.Tile.Letter != wanted {
				return nil, false
			}
		} else {
			index := -1
			for tileIndex, tile := range available {
				if used[tileIndex] || tile.Blank {
					continue
				}
				if tile.Letter == wanted {
					index = tileIndex
					break
				}
			}
			blankIndex := -1
			if index < 0 {
				for tileIndex, tile := range available {
					if !used[tileIndex] && tile.Blank {
						blankIndex = tileIndex
						break
					}
				}
				index = blankIndex
			}
			if index < 0 {
				return nil, false
			}
			used[index] = true
			tile := available[index]
			blankLetter := ""
			if tile.Blank {
				blankLetter = wanted
			}
			placements = append(placements, game.Placement{
				TileID:      tile.ID,
				X:           coord.X,
				Y:           coord.Y,
				BlankLetter: blankLetter,
			})
		}
		coord = game.Coord{X: coord.X + deltaX, Y: coord.Y + deltaY}
	}
	return placements, true
}

func pluralLetters(length int) string {
	if length == 1 {
		return "1 letter"
	}
	return strconv.Itoa(length) + " letters"
}

func findPlayableWithPool(value *game.Game, dict *dictionary.Dictionary, playerIndex int, pool []game.Tile) ([]game.Tile, bool) {
	if value == nil || dict == nil || playerIndex < 0 || playerIndex >= len(value.Players) {
		return nil, false
	}
	player := &value.Players[playerIndex]
	backup := player.Rack
	defer func() {
		player.Rack = backup
	}()
	poolByID := make(map[game.TileID]game.Tile, len(pool))
	for _, tile := range pool {
		poolByID[tile.ID] = tile
	}
	board := game.BoardMap(value.Board)
	for _, entry := range dict.Entries() {
		length := utf8.RuneCountInString(entry.Word)
		if length < 2 || length > value.Config.MaxWordLength {
			continue
		}
		for _, direction := range []game.Direction{game.DirectionHorizontal, game.DirectionVertical} {
			for startY := 0; startY < game.BoardSize; startY++ {
				for startX := 0; startX < game.BoardSize; startX++ {
					start := game.Coord{X: startX, Y: startY}
					if direction == game.DirectionHorizontal && startX+length > game.BoardSize {
						continue
					}
					if direction == game.DirectionVertical && startY+length > game.BoardSize {
						continue
					}
					placements, ok := matchWord(pool, board, entry.Word, start, direction)
					if !ok || len(placements) == 0 {
						continue
					}
					used := make([]game.Tile, 0, len(placements))
					valid := true
					for _, placement := range placements {
						tile, exists := poolByID[placement.TileID]
						if !exists {
							valid = false
							break
						}
						used = append(used, tile)
					}
					if !valid {
						continue
					}
					player.Rack = used
					_, _, _, err := game.ValidatePlacement(value, dict, placements)
					if err == nil {
						return used, true
					}
				}
			}
		}
	}
	return nil, false
}
