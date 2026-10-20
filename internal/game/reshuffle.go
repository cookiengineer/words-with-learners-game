package game

func SwapRackWithBag(value *Game, playerIndex int, attempt int) ([]TileID, []TileID, bool) {
	if value == nil || value.Status != StatusPlaying {
		return nil, nil, false
	}
	if playerIndex < 0 || playerIndex >= len(value.Players) {
		return nil, nil, false
	}
	player := &value.Players[playerIndex]
	if len(player.Rack) == 0 || len(value.Bag) < len(player.Rack) {
		return nil, nil, false
	}
	oldRack := append([]Tile(nil), player.Rack...)
	drawn := DrawTiles(&value.Bag, len(oldRack))
	value.Bag = append(value.Bag, oldRack...)
	player.Rack = drawn
	seed := MoveSeed(value.Config.Seed, value.MoveCount*1000+attempt, "auto-reshuffle")
	ShuffleTiles(NewRNG(MoveSeed(seed, 0, "bag")), value.Bag)
	ShuffleTiles(NewRNG(MoveSeed(seed, 0, "rack")), player.Rack)
	return tileIDs(oldRack), tileIDs(drawn), true
}

func EnsureRackContains(value *Game, playerIndex int, wanted []Tile) ([]TileID, []TileID, bool) {
	if value == nil || value.Status != StatusPlaying {
		return nil, nil, false
	}
	if playerIndex < 0 || playerIndex >= len(value.Players) {
		return nil, nil, false
	}
	player := &value.Players[playerIndex]
	if len(wanted) == 0 || len(wanted) > len(player.Rack) {
		return nil, nil, false
	}
	originalRack := append([]Tile(nil), player.Rack...)
	originalBag := append([]Tile(nil), value.Bag...)
	rackByID := make(map[TileID]Tile, len(originalRack))
	for _, tile := range originalRack {
		rackByID[tile.ID] = tile
	}
	bagByID := make(map[TileID]Tile, len(originalBag))
	for _, tile := range originalBag {
		bagByID[tile.ID] = tile
	}
	wantedSet := make(map[TileID]bool, len(wanted))
	newRack := make([]Tile, 0, len(originalRack))
	for _, tile := range wanted {
		if wantedSet[tile.ID] {
			continue
		}
		actual, inRack := rackByID[tile.ID]
		if !inRack {
			var inBag bool
			actual, inBag = bagByID[tile.ID]
			if !inBag {
				return nil, nil, false
			}
		}
		wantedSet[tile.ID] = true
		newRack = append(newRack, actual)
	}
	if len(newRack) > len(originalRack) {
		return nil, nil, false
	}
	kept := make(map[TileID]bool, len(originalRack))
	for _, tile := range newRack {
		kept[tile.ID] = true
	}
	for _, tile := range originalRack {
		if len(newRack) >= len(originalRack) {
			break
		}
		if kept[tile.ID] {
			continue
		}
		newRack = append(newRack, tile)
		kept[tile.ID] = true
	}
	newBag := make([]Tile, 0, len(originalBag))
	for _, tile := range originalBag {
		if wantedSet[tile.ID] {
			if _, inRack := rackByID[tile.ID]; !inRack {
				continue
			}
		}
		newBag = append(newBag, tile)
	}
	for _, tile := range originalRack {
		if !kept[tile.ID] {
			newBag = append(newBag, tile)
		}
	}
	seed := MoveSeed(value.Config.Seed, value.MoveCount, "targeted-reshuffle")
	ShuffleTiles(NewRNG(MoveSeed(seed, 0, "bag")), newBag)
	ShuffleTiles(NewRNG(MoveSeed(seed, 0, "rack")), newRack)
	value.Bag = newBag
	player.Rack = newRack
	return diffTileIDs(originalRack, newRack), diffTileIDs(newRack, originalRack), true
}

func diffTileIDs(from []Tile, to []Tile) []TileID {
	toSet := make(map[TileID]bool, len(to))
	for _, tile := range to {
		toSet[tile.ID] = true
	}
	result := make([]TileID, 0)
	for _, tile := range from {
		if !toSet[tile.ID] {
			result = append(result, tile.ID)
		}
	}
	return result
}
