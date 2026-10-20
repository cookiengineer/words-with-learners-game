package server

import (
	"github.com/cookiengineer/words-with-learners-app/internal/dictionary"
	"github.com/cookiengineer/words-with-learners-app/internal/game"
)

const autoReshuffleAttempts = 40

func ensurePlayable(value *game.Game, dict *dictionary.Dictionary) *game.AutoReshuffle {
	if value == nil || dict == nil || value.Status != game.StatusPlaying {
		return nil
	}
	if value.Current < 0 || value.Current >= len(value.Players) {
		return nil
	}
	if findHintFor(value, dict, value.Current, true).Found {
		return nil
	}
	player := &value.Players[value.Current]
	if len(player.Rack) == 0 {
		return nil
	}
	initialRack := append([]game.Tile(nil), player.Rack...)
	initialBag := append([]game.Tile(nil), value.Bag...)
	randomSwaps := 0
	for randomSwaps < autoReshuffleAttempts {
		if len(value.Bag) < len(player.Rack) {
			break
		}
		if _, _, ok := game.SwapRackWithBag(value, value.Current, randomSwaps+1); !ok {
			break
		}
		randomSwaps++
		if findHintFor(value, dict, value.Current, true).Found {
			return reshuffleRecord(player, initialRack, value.MoveCount, randomSwaps, nil)
		}
	}
	pool := append(append([]game.Tile(nil), value.Bag...), player.Rack...)
	if used, ok := findPlayableWithPool(value, dict, value.Current, pool); ok {
		if _, _, ensured := game.EnsureRackContains(value, value.Current, used); ensured {
			targeted := make([]game.TileID, len(used))
			for index := range used {
				targeted[index] = used[index].ID
			}
			if findHintFor(value, dict, value.Current, true).Found {
				return reshuffleRecord(player, initialRack, value.MoveCount, randomSwaps, targeted)
			}
		}
	}
	value.Bag = initialBag
	player.Rack = initialRack
	return nil
}

func reshuffleRecord(player *game.Player, initial []game.Tile, moveCount int, randomSwaps int, targeted []game.TileID) *game.AutoReshuffle {
	returned, drawn := tileDiff(initial, player.Rack)
	attempts := randomSwaps
	if len(targeted) > 0 {
		attempts++
	}
	return &game.AutoReshuffle{
		PlayerID:    player.ID,
		Returned:    returned,
		Drawn:       drawn,
		Targeted:    append([]game.TileID(nil), targeted...),
		Attempts:    attempts,
		RandomSwaps: randomSwaps,
		MoveCount:   moveCount,
	}
}

func tileDiff(from []game.Tile, to []game.Tile) ([]game.TileID, []game.TileID) {
	fromSet := make(map[game.TileID]bool, len(from))
	toSet := make(map[game.TileID]bool, len(to))
	for _, tile := range from {
		fromSet[tile.ID] = true
	}
	for _, tile := range to {
		toSet[tile.ID] = true
	}
	returned := make([]game.TileID, 0)
	drawn := make([]game.TileID, 0)
	for _, tile := range from {
		if !toSet[tile.ID] {
			returned = append(returned, tile.ID)
		}
	}
	for _, tile := range to {
		if !fromSet[tile.ID] {
			drawn = append(drawn, tile.ID)
		}
	}
	return returned, drawn
}
