package game

import (
	"time"
)

func ApplyMove(game *Game, lexicon Lexicon, playerID PlayerID, request MoveRequest, now time.Time) (MoveRecord, error) {
	if game == nil {
		return MoveRecord{}, RuleErrorf("invalid_state", "game is missing")
	}
	if err := request.Validate(); err != nil {
		return MoveRecord{}, RuleErrorf("invalid_move", "%s", err.Error())
	}
	for _, record := range game.Moves {
		if record.MoveID == request.MoveID {
			return record, nil
		}
	}
	if game.Status != StatusPlaying {
		return MoveRecord{}, RuleErrorf("game_finished", "the game has already finished")
	}
	if request.ExpectedRevision != game.Revision {
		return MoveRecord{}, RuleErrorf("stale_revision", "the game state changed; refresh and try again")
	}
	playerIndex := game.PlayerIndexByID(playerID)
	if playerIndex < 0 {
		return MoveRecord{}, RuleErrorf("not_a_player", "you are not part of this game")
	}
	if playerIndex != game.Current {
		return MoveRecord{}, RuleErrorf("not_your_turn", "it is not your turn")
	}
	player := &game.Players[playerIndex]
	record := MoveRecord{
		MoveID:   request.MoveID,
		Seq:      len(game.Moves) + 1,
		PlayerID: playerID,
		Kind:     request.Kind,
		At:       now.UTC(),
	}

	switch request.Kind {
	case MovePlace:
		rackBefore := len(player.Rack)
		resolved, words, placementScore, err := ValidatePlacement(game, lexicon, request.Placements)
		if err != nil {
			return MoveRecord{}, err
		}
		removeTiles(player, request.Placements)
		for _, placed := range resolved {
			game.Board = append(game.Board, PlacedTile{Tile: placed.Tile, X: placed.X, Y: placed.Y})
		}
		score := placementScore
		if rackBefore == len(resolved) && rackBefore > 0 {
			score += game.Config.BingoBonus
		}
		player.Score += score
		record.Placements = resolved
		record.Words = words
		record.Score = score
		game.Scoreless = 0

		needed := game.Config.RackSize() - len(player.Rack)
		drawn := DrawTiles(&game.Bag, needed)
		player.Rack = append(player.Rack, drawn...)
		if len(drawn) > 0 {
			ShuffleTiles(NewRNG(MoveSeed(game.Config.Seed, game.MoveCount+1, "rack")), player.Rack)
		}
		record.Drawn = tileIDs(drawn)

		if len(game.Bag) == 0 && len(player.Rack) == 0 {
			finishWithAward(game, playerIndex)
		} else {
			advanceTurn(game)
		}
	case MovePass:
		game.Scoreless++
		if game.Scoreless >= 3 && !allScoresZero(game) {
			finishByScore(game)
		} else {
			advanceTurn(game)
		}
	case MoveExchange:
		drawn, err := exchangeTiles(game, player, request.Exchange)
		if err != nil {
			return MoveRecord{}, err
		}
		record.Exchange = append([]TileID(nil), request.Exchange...)
		record.Drawn = tileIDs(drawn)
		game.Scoreless++
		if game.Scoreless >= 3 && !allScoresZero(game) {
			finishByScore(game)
		} else {
			advanceTurn(game)
		}
	}

	record.ScoresAfter = scores(game)
	record.BagCount = len(game.Bag)
	record.BoardHash = BoardHash(game.Board)
	game.Revision++
	game.MoveCount++
	game.UpdatedAt = now.UTC()
	game.Board = SortedBoard(game.Board)
	game.Moves = append(game.Moves, record)
	return record, nil
}

func removeTiles(player *Player, placements []Placement) {
	remove := make(map[TileID]bool, len(placements))
	for _, placement := range placements {
		remove[placement.TileID] = true
	}
	rack := make([]Tile, 0, len(player.Rack))
	for _, tile := range player.Rack {
		if !remove[tile.ID] {
			rack = append(rack, tile)
		}
	}
	player.Rack = rack
}

func exchangeTiles(game *Game, player *Player, ids []TileID) ([]Tile, error) {
	if len(ids) > len(player.Rack) {
		return nil, RuleErrorf("invalid_exchange", "you cannot exchange more tiles than you hold")
	}
	if len(ids) > len(game.Bag) {
		return nil, RuleErrorf("bag_too_small", "the bag does not have enough tiles left to exchange")
	}
	exchange := make(map[TileID]bool, len(ids))
	exchangeTiles := make([]Tile, 0, len(ids))
	for _, id := range ids {
		if exchange[id] {
			return nil, RuleErrorf("invalid_exchange", "tile %q was selected twice", id)
		}
		exchange[id] = true
		found := false
		for _, tile := range player.Rack {
			if tile.ID == id {
				exchangeTiles = append(exchangeTiles, tile)
				found = true
				break
			}
		}
		if !found {
			return nil, RuleErrorf("foreign_tile", "tile %q is not in your rack", id)
		}
	}
	remove := make(map[TileID]bool, len(ids))
	for _, id := range ids {
		remove[id] = true
	}
	rack := make([]Tile, 0, len(player.Rack))
	for _, tile := range player.Rack {
		if !remove[tile.ID] {
			rack = append(rack, tile)
		}
	}
	drawn := DrawTiles(&game.Bag, len(ids))
	rack = append(rack, drawn...)
	ShuffleTiles(NewRNG(MoveSeed(game.Config.Seed, game.MoveCount+1, "rack")), rack)
	player.Rack = rack
	game.Bag = append(game.Bag, exchangeTiles...)
	ShuffleTiles(NewRNG(MoveSeed(game.Config.Seed, game.MoveCount+1, "exchange")), game.Bag)
	return drawn, nil
}

func advanceTurn(game *Game) {
	if len(game.Players) == 0 {
		game.Current = 0
		return
	}
	game.Current = (game.Current + 1) % len(game.Players)
}

func scores(game *Game) []int {
	result := make([]int, len(game.Players))
	for index := range game.Players {
		result[index] = game.Players[index].Score
	}
	return result
}

func tileIDs(tiles []Tile) []TileID {
	result := make([]TileID, len(tiles))
	for index := range tiles {
		result[index] = tiles[index].ID
	}
	return result
}

func allScoresZero(game *Game) bool {
	for _, player := range game.Players {
		if player.Score != 0 {
			return false
		}
	}
	return true
}

func finishWithAward(game *Game, finisher int) {
	total := 0
	for index := range game.Players {
		if index == finisher {
			continue
		}
		for _, tile := range game.Players[index].Rack {
			total += tile.Value
		}
	}
	for index := range game.Players {
		if index == finisher {
			continue
		}
		for _, tile := range game.Players[index].Rack {
			game.Players[index].Score -= tile.Value
		}
	}
	game.Players[finisher].Score += total
	finishByScore(game)
}

func finishByScore(game *Game) {
	game.Status = StatusFinished
	if len(game.Players) == 0 {
		game.Winner = nil
		return
	}
	best := 0
	tied := false
	for index := 1; index < len(game.Players); index++ {
		if game.Players[index].Score > game.Players[best].Score {
			best = index
			tied = false
		} else if game.Players[index].Score == game.Players[best].Score {
			tied = true
		}
	}
	if tied {
		game.Winner = nil
	} else {
		winner := game.Players[best].ID
		game.Winner = &winner
	}
}
