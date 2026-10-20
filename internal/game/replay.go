package game

import (
	"fmt"
	"reflect"
)

func Replay(game *Game, lexicon Lexicon) error {
	if game == nil {
		return RuleErrorf("invalid_state", "game is missing")
	}
	if len(game.Specs) == 0 {
		return RuleErrorf("invalid_state", "game has no alphabet snapshot")
	}
	seeds := make([]PlayerSeed, len(game.Players))
	for index := range game.Players {
		seeds[index] = PlayerSeed{ID: game.Players[index].ID, Name: game.Players[index].Name}
	}
	replay, err := NewGame(game.ID, game.Config, seeds, game.Specs, game.CreatedAt)
	if err != nil {
		return err
	}
	reshuffles := append([]AutoReshuffle(nil), game.Reshuffles...)
	reshuffleIndex := 0
	applyReshuffles := func(moveCount int) error {
		for reshuffleIndex < len(reshuffles) {
			record := reshuffles[reshuffleIndex]
			if record.MoveCount > moveCount {
				break
			}
			if record.MoveCount < moveCount {
				return fmt.Errorf("reshuffle for move %d is out of order", record.MoveCount)
			}
			if err := applyAutoReshuffle(replay, record); err != nil {
				return err
			}
			reshuffleIndex++
		}
		return nil
	}
	if err := applyReshuffles(0); err != nil {
		return err
	}
	for _, original := range game.Moves {
		request := MoveRequest{
			MoveID:           original.MoveID,
			ExpectedRevision: replay.Revision,
			Kind:             original.Kind,
		}
		switch original.Kind {
		case MovePlace:
			for _, placed := range original.Placements {
				blankLetter := ""
				if placed.Tile.Blank {
					blankLetter = placed.Tile.Letter
				}
				request.Placements = append(request.Placements, Placement{
					TileID:      placed.Tile.ID,
					X:           placed.X,
					Y:           placed.Y,
					BlankLetter: blankLetter,
				})
			}
		case MoveExchange:
			request.Exchange = append([]TileID(nil), original.Exchange...)
		}
		replayed, err := ApplyMove(replay, lexicon, original.PlayerID, request, original.At)
		if err != nil {
			return fmt.Errorf("replay move %d (%s): %w", original.Seq, original.MoveID, err)
		}
		if replayed.BoardHash != original.BoardHash {
			return fmt.Errorf("replay move %d: board hash mismatch", original.Seq)
		}
		if !reflect.DeepEqual(replayed.ScoresAfter, original.ScoresAfter) {
			return fmt.Errorf("replay move %d: score mismatch", original.Seq)
		}
		if replayed.Score != original.Score {
			return fmt.Errorf("replay move %d: move score mismatch", original.Seq)
		}
		if err := applyReshuffles(replay.MoveCount); err != nil {
			return fmt.Errorf("replay move %d: %w", original.Seq, err)
		}
	}
	if reshuffleIndex < len(reshuffles) {
		return fmt.Errorf("replay left %d reshuffles unapplied", len(reshuffles)-reshuffleIndex)
	}
	if replay.Status != game.Status {
		return fmt.Errorf("replay status mismatch: got %s, want %s", replay.Status, game.Status)
	}
	return nil
}

func applyAutoReshuffle(value *Game, record AutoReshuffle) error {
	playerIndex := value.PlayerIndexByID(record.PlayerID)
	if playerIndex < 0 {
		return fmt.Errorf("reshuffle player %s is not in the game", record.PlayerID)
	}
	for attempt := 1; attempt <= record.RandomSwaps; attempt++ {
		if _, _, ok := SwapRackWithBag(value, playerIndex, attempt); !ok {
			return fmt.Errorf("reshuffle swap %d could not be reproduced", attempt)
		}
	}
	if len(record.Targeted) > 0 {
		tilesByID := make(map[TileID]Tile, len(value.Players[playerIndex].Rack)+len(value.Bag))
		for _, tile := range value.Players[playerIndex].Rack {
			tilesByID[tile.ID] = tile
		}
		for _, tile := range value.Bag {
			tilesByID[tile.ID] = tile
		}
		targeted := make([]Tile, 0, len(record.Targeted))
		for _, id := range record.Targeted {
			tile, ok := tilesByID[id]
			if !ok {
				return fmt.Errorf("reshuffle tile %s is missing", id)
			}
			targeted = append(targeted, tile)
		}
		if _, _, ok := EnsureRackContains(value, playerIndex, targeted); !ok {
			return fmt.Errorf("targeted reshuffle could not be reproduced")
		}
	}
	return nil
}
