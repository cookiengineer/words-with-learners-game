package game

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

type Lexicon interface {
	NormalizeLetter(letter string) string
	NormalizeWord(word string) string
	HasLetter(letter string) bool
	ContainsWord(word string) bool
}

type wordRef struct {
	start     Coord
	direction Direction
}

func (ref wordRef) key() string {
	return fmt.Sprintf("%s:%s", ref.start.Key(), ref.direction)
}

func (ref wordRef) less(other wordRef) bool {
	if ref.start.Y == other.start.Y && ref.start.X == other.start.X {
		return ref.direction < other.direction
	}
	if ref.start.Y == other.start.Y {
		return ref.start.X < other.start.X
	}
	return ref.start.Y < other.start.Y
}

func ValidatePlacement(game *Game, lexicon Lexicon, placements []Placement) ([]ResolvedPlacement, []FormedWord, int, error) {
	if game == nil {
		return nil, nil, 0, RuleErrorf("invalid_state", "game or lexicon is missing")
	}
	return ValidatePlacementFor(game, lexicon, game.Current, placements)
}

func ValidatePlacementFor(game *Game, lexicon Lexicon, playerIndex int, placements []Placement) ([]ResolvedPlacement, []FormedWord, int, error) {
	if game == nil || lexicon == nil {
		return nil, nil, 0, RuleErrorf("invalid_state", "game or lexicon is missing")
	}
	if game.Status != StatusPlaying {
		return nil, nil, 0, RuleErrorf("game_finished", "the game has already finished")
	}
	if len(placements) == 0 {
		return nil, nil, 0, RuleErrorf("empty_move", "place at least one tile")
	}
	if len(placements) > game.Config.RackSize() {
		return nil, nil, 0, RuleErrorf("too_many_tiles", "you cannot place more than %d tiles", game.Config.RackSize())
	}

	if playerIndex < 0 || playerIndex >= len(game.Players) {
		return nil, nil, 0, RuleErrorf("invalid_state", "player is missing")
	}
	player := &game.Players[playerIndex]

	rack := make(map[TileID]Tile, len(player.Rack))
	for _, tile := range player.Rack {
		rack[tile.ID] = tile
	}

	board := BoardMap(game.Board)
	trial := make(map[Coord]PlacedTile, len(game.Board)+len(placements))
	for coord, placed := range board {
		trial[coord] = placed
	}

	resolved := make([]ResolvedPlacement, 0, len(placements))
	coords := make([]Coord, 0, len(placements))
	newAt := make(map[Coord]bool, len(placements))
	usedTiles := make(map[TileID]bool, len(placements))

	for _, placement := range placements {
		if usedTiles[placement.TileID] {
			return nil, nil, 0, RuleErrorf("duplicate_tile", "tile %q was used twice", placement.TileID)
		}
		tile, ok := rack[placement.TileID]
		if !ok {
			return nil, nil, 0, RuleErrorf("foreign_tile", "tile %q is not in your rack", placement.TileID)
		}
		usedTiles[placement.TileID] = true

		coord := Coord{X: placement.X, Y: placement.Y}
		if !coord.Valid() {
			return nil, nil, 0, RuleErrorf("out_of_bounds", "tile position %d,%d is outside the board", placement.X, placement.Y)
		}
		if _, occupied := board[coord]; occupied {
			return nil, nil, 0, RuleErrorf("occupied", "square %d,%d is already occupied", placement.X, placement.Y)
		}
		if newAt[coord] {
			return nil, nil, 0, RuleErrorf("duplicate_position", "two tiles were placed on %d,%d", placement.X, placement.Y)
		}

		resolvedTile := tile
		if tile.Blank {
			letter := lexicon.NormalizeLetter(placement.BlankLetter)
			if letter == "" || utf8.RuneCountInString(letter) != 1 {
				return nil, nil, 0, RuleErrorf("blank_letter", "choose one letter for the blank tile")
			}
			if !lexicon.HasLetter(letter) {
				return nil, nil, 0, RuleErrorf("blank_letter", "letter %q is not part of this alphabet", letter)
			}
			resolvedTile.Letter = letter
			resolvedTile.Value = 0
		} else if strings.TrimSpace(placement.BlankLetter) != "" {
			return nil, nil, 0, RuleErrorf("invalid_move", "only a blank tile can choose a letter")
		}
		if resolvedTile.Letter == "" {
			return nil, nil, 0, RuleErrorf("invalid_tile", "tile %q has no letter", tile.ID)
		}

		resolved = append(resolved, ResolvedPlacement{Tile: resolvedTile, X: coord.X, Y: coord.Y})
		coords = append(coords, coord)
		newAt[coord] = true
		trial[coord] = PlacedTile{Tile: resolvedTile, X: coord.X, Y: coord.Y}
	}

	firstMove := len(game.Board) == 0
	if firstMove {
		if len(placements) < 2 {
			return nil, nil, 0, RuleErrorf("first_move", "the first word needs at least two tiles")
		}
		center := CenterCoord()
		coversCenter := false
		for _, coord := range coords {
			if coord == center {
				coversCenter = true
				break
			}
		}
		if !coversCenter {
			return nil, nil, 0, RuleErrorf("first_move", "the first word must cover the center square")
		}
	} else {
		connected := false
		for _, coord := range coords {
			neighbors := []Coord{
				{X: coord.X - 1, Y: coord.Y},
				{X: coord.X + 1, Y: coord.Y},
				{X: coord.X, Y: coord.Y - 1},
				{X: coord.X, Y: coord.Y + 1},
			}
			for _, neighbor := range neighbors {
				if _, ok := board[neighbor]; ok {
					connected = true
					break
				}
			}
			if connected {
				break
			}
		}
		if !connected {
			return nil, nil, 0, RuleErrorf("disconnected", "the word must connect to the existing board")
		}
	}

	primaryDirection, err := placementDirection(coords)
	if err != nil {
		return nil, nil, 0, err
	}
	if err := ensureNoGaps(trial, coords, primaryDirection); err != nil {
		return nil, nil, 0, err
	}

	refs := make(map[string]wordRef)
	for _, coord := range coords {
		for _, direction := range []Direction{DirectionHorizontal, DirectionVertical} {
			start := WordStart(trial, coord, direction)
			word, length := RunLength(trial, start, direction)
			if length < 2 {
				continue
			}
			ref := wordRef{start: start, direction: direction}
			refs[ref.key()] = ref
			_ = word
		}
	}
	if len(refs) == 0 {
		return nil, nil, 0, RuleErrorf("no_word", "that placement does not form a word")
	}

	sortedRefs := make([]wordRef, 0, len(refs))
	for _, ref := range refs {
		sortedRefs = append(sortedRefs, ref)
	}
	sort.Slice(sortedRefs, func(i, j int) bool {
		return sortedRefs[i].less(sortedRefs[j])
	})

	primaryRef, err := choosePrimaryRef(sortedRefs, trial, coords, primaryDirection)
	if err != nil {
		return nil, nil, 0, err
	}

	layout := WWFLayout()
	words := make([]FormedWord, 0, len(sortedRefs))
	totalScore := 0
	for _, ref := range sortedRefs {
		rawWord, length := RunLength(trial, ref.start, ref.direction)
		if length < 2 || length > game.Config.MaxWordLength {
			return nil, nil, 0, RuleErrorf("word_length", "word %q must be between 2 and %d letters", rawWord, game.Config.MaxWordLength)
		}
		word := lexicon.NormalizeWord(rawWord)
		if word == "" || !lexicon.ContainsWord(word) {
			return nil, nil, 0, RuleErrorf("unknown_word", "%q is not in the %s %s dictionary", word, game.Config.Language.Name(), game.Config.Level)
		}
		score, _ := ScoreWord(layout, trial, ref.start, ref.direction, newAt)
		totalScore += score
		words = append(words, FormedWord{
			X:         ref.start.X,
			Y:         ref.start.Y,
			Direction: ref.direction,
			Word:      word,
			Score:     score,
			Primary:   ref.key() == primaryRef.key(),
		})
	}

	return resolved, words, totalScore, nil
}

func placementDirection(coords []Coord) (Direction, error) {
	if len(coords) == 0 {
		return "", RuleErrorf("empty_move", "place at least one tile")
	}
	sameRow := true
	sameColumn := true
	for _, coord := range coords[1:] {
		if coord.Y != coords[0].Y {
			sameRow = false
		}
		if coord.X != coords[0].X {
			sameColumn = false
		}
	}
	switch {
	case sameRow && sameColumn:
		return "", nil
	case sameRow:
		return DirectionHorizontal, nil
	case sameColumn:
		return DirectionVertical, nil
	default:
		return "", RuleErrorf("not_in_line", "all tiles must be placed in one row or one column")
	}
}

func ensureNoGaps(board map[Coord]PlacedTile, coords []Coord, direction Direction) error {
	if len(coords) < 2 {
		return nil
	}
	sorted := append([]Coord(nil), coords...)
	sort.Slice(sorted, func(i, j int) bool {
		if direction == DirectionHorizontal {
			return sorted[i].X < sorted[j].X
		}
		return sorted[i].Y < sorted[j].Y
	})
	first := sorted[0]
	last := sorted[len(sorted)-1]
	for {
		if _, ok := board[first]; !ok {
			return RuleErrorf("gap", "the placed tiles must not leave a gap")
		}
		if first == last {
			break
		}
		if direction == DirectionHorizontal {
			first.X++
		} else {
			first.Y++
		}
	}
	return nil
}

func choosePrimaryRef(refs []wordRef, board map[Coord]PlacedTile, coords []Coord, direction Direction) (wordRef, error) {
	if len(refs) == 1 {
		return refs[0], nil
	}
	if direction != "" {
		for _, ref := range refs {
			if ref.direction == direction {
				return ref, nil
			}
		}
	}
	best := refs[0]
	bestLength := 0
	for _, ref := range refs {
		word, length := RunLength(board, ref.start, ref.direction)
		if length > bestLength || (length == bestLength && ref.less(best)) {
			_ = word
			best = ref
			bestLength = length
		}
	}
	if bestLength < 2 {
		return wordRef{}, RuleErrorf("no_word", "that placement does not form a word")
	}
	return best, nil
}

func ScoreWord(layout Layout, board map[Coord]PlacedTile, start Coord, direction Direction, newAt map[Coord]bool) (int, string) {
	deltaX, deltaY := 0, 0
	if direction == DirectionHorizontal {
		deltaX = 1
	} else {
		deltaY = 1
	}
	coord := start
	sum := 0
	wordMultiplier := 1
	builder := strings.Builder{}
	for {
		placed, ok := board[coord]
		if !ok {
			break
		}
		letterValue := placed.Tile.Value
		if newAt[coord] {
			switch layout.At(coord) {
			case BonusDL:
				letterValue *= 2
			case BonusTL:
				letterValue *= 3
			case BonusDW:
				wordMultiplier *= 2
			case BonusTW:
				wordMultiplier *= 3
			}
		}
		sum += letterValue
		builder.WriteString(placed.Tile.Letter)
		coord = Coord{X: coord.X + deltaX, Y: coord.Y + deltaY}
	}
	return sum * wordMultiplier, builder.String()
}
