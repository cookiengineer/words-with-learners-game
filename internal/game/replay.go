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
	}
	if replay.Status != game.Status {
		return fmt.Errorf("replay status mismatch: got %s, want %s", replay.Status, game.Status)
	}
	return nil
}
