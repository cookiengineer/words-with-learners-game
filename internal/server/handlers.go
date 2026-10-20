package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/cookiengineer/gows"
	"github.com/cookiengineer/words-with-learners-app/internal/game"
	"github.com/cookiengineer/words-with-learners-app/internal/store"
	"github.com/cookiengineer/words-with-learners-app/internal/view"
)

const maxMessageSize = 64 * 1024

func (server *Server) handleMessage(conn *client, raw []byte) {
	if len(raw) > maxMessageSize {
		if conn.socket != nil {
			conn.socket.Close(gows.StatusMessageTooBig, "message too large")
		}
		return
	}
	var envelope Envelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		conn.send(TypeError, "", ErrorPayload{Code: "invalid_json", Message: "could not parse message"})
		return
	}
	switch envelope.Type {
	case TypePlayersSearch:
		server.handlePlayersSearch(conn, envelope)
	case TypeGamesList:
		server.handleGamesList(conn, envelope)
	case TypeGameCreate:
		server.handleGameCreate(conn, envelope)
	case TypeGameState:
		server.handleGameState(conn, envelope)
	case TypeMovePreview:
		server.handleMovePreview(conn, envelope)
	case TypeMoveSubmit:
		server.handleMoveSubmit(conn, envelope)
	case TypeHintGet:
		server.handleHintGet(conn, envelope)
	default:
		conn.send(TypeError, envelope.RequestID, ErrorPayload{Code: "unknown_type", Message: fmt.Sprintf("unknown message type %q", envelope.Type)})
	}
}

func decodePayload(envelope Envelope, target any) error {
	if len(envelope.Payload) == 0 {
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(envelope.Payload))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func (server *Server) handlePlayersSearch(conn *client, envelope Envelope) {
	var request SearchPlayersRequest
	if err := decodePayload(envelope, &request); err != nil {
		conn.send(TypeError, envelope.RequestID, ErrorPayload{Code: "invalid_payload", Message: err.Error()})
		return
	}
	profiles, total := server.store.SearchProfiles(request.Query, request.Offset, request.Limit, conn.profile.ID)
	players := make([]PlayerResult, len(profiles))
	for index := range profiles {
		players[index] = PlayerResult{
			ID:       profiles[index].ID,
			Username: profiles[index].Username,
		}
	}
	next := request.Offset + len(players)
	if next >= total {
		next = 0
	}
	conn.send(TypePlayersSearch, envelope.RequestID, SearchPlayersResponse{Players: players, Total: total, NextOffset: next})
}

func (server *Server) handleGamesList(conn *client, envelope Envelope) {
	conn.send(TypeGamesList, envelope.RequestID, GamesListResponse{Games: server.gameCards(conn.profile.ID)})
}

func (server *Server) handleGameCreate(conn *client, envelope Envelope) {
	var request CreateGameRequest
	if err := decodePayload(envelope, &request); err != nil {
		conn.send(TypeError, envelope.RequestID, ErrorPayload{Code: "invalid_payload", Message: err.Error()})
		return
	}
	if request.PlayerCount < game.MinPlayers || request.PlayerCount > game.MaxPlayers {
		conn.send(TypeError, envelope.RequestID, ErrorPayload{Code: "invalid_players", Message: fmt.Sprintf("choose %d to %d players", game.MinPlayers, game.MaxPlayers)})
		return
	}
	if len(request.Opponents) != request.PlayerCount-1 {
		conn.send(TypeError, envelope.RequestID, ErrorPayload{Code: "invalid_players", Message: "select exactly the requested number of opponents"})
		return
	}
	seen := map[game.PlayerID]bool{conn.profile.ID: true}
	opponents := make([]*store.Profile, 0, len(request.Opponents))
	for _, id := range request.Opponents {
		if seen[id] {
			conn.send(TypeError, envelope.RequestID, ErrorPayload{Code: "duplicate_player", Message: "each player can only occupy one seat"})
			return
		}
		seen[id] = true
		profile, ok := server.store.Profile(id)
		if !ok {
			conn.send(TypeError, envelope.RequestID, ErrorPayload{Code: "unknown_player", Message: "one of the selected players does not exist"})
			return
		}
		opponents = append(opponents, profile)
	}
	dict, err := server.dictionaries.Get(request.Language, request.Level)
	if err != nil {
		conn.send(TypeError, envelope.RequestID, ErrorPayload{Code: "unknown_dictionary", Message: err.Error()})
		return
	}
	seed, err := game.RandomSeed()
	if err != nil {
		conn.send(TypeError, envelope.RequestID, ErrorPayload{Code: "internal_error", Message: "could not create a game seed"})
		return
	}
	config := game.Config{
		Language:      request.Language,
		Level:         request.Level,
		MaxWordLength: request.MaxWordLength,
		PlayerCount:   request.PlayerCount,
		BoardSize:     game.BoardSize,
		BingoBonus:    request.MaxWordLength * 5,
		Seed:          seed,
	}
	seeds := []game.PlayerSeed{{ID: conn.profile.ID, Name: conn.profile.Username}}
	for _, opponent := range opponents {
		seeds = append(seeds, game.PlayerSeed{ID: opponent.ID, Name: opponent.Username})
	}
	id, err := store.NewID()
	if err != nil {
		conn.send(TypeError, envelope.RequestID, ErrorPayload{Code: "internal_error", Message: "could not create a game id"})
		return
	}
	now := time.Now()
	value, err := game.NewGame(game.GameID(id), config, seeds, dict.Specs(), now)
	if err != nil {
		conn.send(TypeError, envelope.RequestID, ErrorPayload{Code: "invalid_game", Message: err.Error()})
		return
	}
	if reshuffle := ensurePlayable(value, dict); reshuffle != nil {
		reshuffle.Revision = value.Revision
		value.LastReshuffle = reshuffle
		value.Reshuffles = append(value.Reshuffles, *reshuffle)
	}
	if err := server.store.SaveGame(value); err != nil {
		conn.send(TypeError, envelope.RequestID, ErrorPayload{Code: "internal_error", Message: "could not save the game"})
		return
	}
	session := &gameSession{value: value}
	server.gamesMutex.Lock()
	server.games[value.ID] = session
	server.gamesMutex.Unlock()
	snapshot, snapshotErr := cloneGame(value)
	if snapshotErr != nil {
		snapshot = value
	}
	conn.send(TypeGameState, envelope.RequestID, view.Build(snapshot, conn.profile.ID, server.onlinePlayers()))
	server.broadcastGame(snapshot)
	server.broadcastGamesChanged()
}

func (server *Server) handleGameState(conn *client, envelope Envelope) {
	var request GameStateRequest
	if err := decodePayload(envelope, &request); err != nil {
		conn.send(TypeError, envelope.RequestID, ErrorPayload{Code: "invalid_payload", Message: err.Error()})
		return
	}
	session, ok := server.session(request.GameID)
	if !ok {
		conn.send(TypeError, envelope.RequestID, ErrorPayload{Code: "not_found", Message: "game not found"})
		return
	}
	session.mutex.Lock()
	value := session.value
	if !value.HasPlayer(conn.profile.ID) {
		session.mutex.Unlock()
		conn.send(TypeError, envelope.RequestID, ErrorPayload{Code: "not_a_player", Message: "you are not part of this game"})
		return
	}
	state := view.Build(value, conn.profile.ID, server.onlinePlayers())
	session.mutex.Unlock()
	conn.send(TypeGameState, envelope.RequestID, state)
}

func (server *Server) handleMovePreview(conn *client, envelope Envelope) {
	var request PreviewRequest
	if err := decodePayload(envelope, &request); err != nil {
		conn.send(TypeError, envelope.RequestID, ErrorPayload{Code: "invalid_payload", Message: err.Error()})
		return
	}
	session, ok := server.session(request.GameID)
	if !ok {
		conn.send(TypeError, envelope.RequestID, ErrorPayload{Code: "not_found", Message: "game not found"})
		return
	}
	session.mutex.Lock()
	value := session.value
	if !value.HasPlayer(conn.profile.ID) {
		session.mutex.Unlock()
		conn.send(TypeError, envelope.RequestID, ErrorPayload{Code: "not_a_player", Message: "you are not part of this game"})
		return
	}
	if value.CurrentPlayer() == nil || value.CurrentPlayer().ID != conn.profile.ID {
		session.mutex.Unlock()
		conn.send(TypeError, envelope.RequestID, ErrorPayload{Code: "not_your_turn", Message: "it is not your turn"})
		return
	}
	if request.Move.Kind != game.MovePlace {
		session.mutex.Unlock()
		conn.send(TypeError, envelope.RequestID, ErrorPayload{Code: "invalid_move", Message: "only placements can be previewed"})
		return
	}
	dict, err := server.dictionaries.Get(value.Config.Language, value.Config.Level)
	if err != nil {
		session.mutex.Unlock()
		conn.send(TypeError, envelope.RequestID, ErrorPayload{Code: "unknown_dictionary", Message: err.Error()})
		return
	}
	_, words, score, err := game.ValidatePlacement(value, dict, request.Move.Placements)
	bingo := err == nil && len(request.Move.Placements) == len(value.Players[value.Current].Rack)
	if err == nil && bingo {
		score += value.Config.BingoBonus
	}
	session.mutex.Unlock()
	if err != nil {
		server.sendRuleError(conn, envelope.RequestID, err)
		return
	}
	conn.send(TypeMovePreview, envelope.RequestID, MovePreview{Score: score, Bingo: bingo, Words: words})
}

func (server *Server) handleMoveSubmit(conn *client, envelope Envelope) {
	var request SubmitRequest
	if err := decodePayload(envelope, &request); err != nil {
		conn.send(TypeError, envelope.RequestID, ErrorPayload{Code: "invalid_payload", Message: err.Error()})
		return
	}
	if request.Move.Kind == game.MoveExchange {
		conn.send(TypeError, envelope.RequestID, ErrorPayload{Code: "exchange_removed", Message: "Exchanging tiles is no longer available; stuck racks are reshuffled automatically."})
		return
	}
	session, ok := server.session(request.GameID)
	if !ok {
		conn.send(TypeError, envelope.RequestID, ErrorPayload{Code: "not_found", Message: "game not found"})
		return
	}
	session.mutex.Lock()
	value := session.value
	if !value.HasPlayer(conn.profile.ID) {
		session.mutex.Unlock()
		conn.send(TypeError, envelope.RequestID, ErrorPayload{Code: "not_a_player", Message: "you are not part of this game"})
		return
	}
	dict, err := server.dictionaries.Get(value.Config.Language, value.Config.Level)
	if err != nil {
		session.mutex.Unlock()
		conn.send(TypeError, envelope.RequestID, ErrorPayload{Code: "unknown_dictionary", Message: err.Error()})
		return
	}
	backup, err := cloneGame(value)
	if err != nil {
		session.mutex.Unlock()
		conn.send(TypeError, envelope.RequestID, ErrorPayload{Code: "internal_error", Message: "could not snapshot the game"})
		return
	}
	record, err := game.ApplyMove(value, dict, conn.profile.ID, request.Move, time.Now())
	if err != nil {
		session.value = backup
		session.mutex.Unlock()
		server.sendRuleError(conn, envelope.RequestID, err)
		return
	}
	if reshuffle := ensurePlayable(value, dict); reshuffle != nil {
		reshuffle.Revision = value.Revision
		value.LastReshuffle = reshuffle
		value.Reshuffles = append(value.Reshuffles, *reshuffle)
		record.AutoReshuffle = reshuffle
		if len(value.Moves) > 0 {
			value.Moves[len(value.Moves)-1].AutoReshuffle = reshuffle
		}
	}
	if err := server.store.SaveGame(value); err != nil {
		session.value = backup
		session.mutex.Unlock()
		conn.send(TypeError, envelope.RequestID, ErrorPayload{Code: "internal_error", Message: "could not save the game"})
		return
	}
	finished := value.Status == game.StatusFinished
	broadcast, cloneErr := cloneGame(value)
	gameID := value.ID
	session.mutex.Unlock()
	conn.send(TypeMoveAccepted, envelope.RequestID, MoveAccepted{GameID: gameID, Move: record})
	if cloneErr == nil {
		server.broadcastGame(broadcast)
	}
	server.broadcastGamesChanged()
	if finished {
		server.logger.Printf("game finished: %s", gameID)
	}
}

func (server *Server) handleHintGet(conn *client, envelope Envelope) {
	var request HintRequest
	if err := decodePayload(envelope, &request); err != nil {
		conn.send(TypeError, envelope.RequestID, ErrorPayload{Code: "invalid_payload", Message: err.Error()})
		return
	}
	session, ok := server.session(request.GameID)
	if !ok {
		conn.send(TypeError, envelope.RequestID, ErrorPayload{Code: "not_found", Message: "game not found"})
		return
	}
	session.mutex.Lock()
	value := session.value
	if !value.HasPlayer(conn.profile.ID) {
		session.mutex.Unlock()
		conn.send(TypeError, envelope.RequestID, ErrorPayload{Code: "not_a_player", Message: "you are not part of this game"})
		return
	}
	if value.CurrentPlayer() == nil || value.CurrentPlayer().ID != conn.profile.ID {
		session.mutex.Unlock()
		conn.send(TypeError, envelope.RequestID, ErrorPayload{Code: "not_your_turn", Message: "it is not your turn"})
		return
	}
	dict, err := server.dictionaries.Get(value.Config.Language, value.Config.Level)
	if err != nil {
		session.mutex.Unlock()
		conn.send(TypeError, envelope.RequestID, ErrorPayload{Code: "unknown_dictionary", Message: err.Error()})
		return
	}
	var hint view.Hint
	var broadcast *game.Game
	if session.hintValid && session.hintRevision == value.Revision && session.hintPlayer == conn.profile.ID {
		hint = session.hintValue
	} else {
		hint = findHint(value, dict)
		if !hint.Found {
			backup, cloneErr := cloneGame(value)
			if cloneErr == nil {
				if reshuffle := ensurePlayable(value, dict); reshuffle != nil {
					value.Revision++
					value.UpdatedAt = time.Now().UTC()
					reshuffle.Revision = value.Revision
					value.LastReshuffle = reshuffle
					value.Reshuffles = append(value.Reshuffles, *reshuffle)
					if saveErr := server.store.SaveGame(value); saveErr != nil {
						session.value = backup
						session.mutex.Unlock()
						conn.send(TypeError, envelope.RequestID, ErrorPayload{Code: "internal_error", Message: "could not save the reshuffled game"})
						return
					}
					hint = findHint(value, dict)
					broadcast, _ = cloneGame(value)
				}
			}
		}
		session.hintRevision = value.Revision
		session.hintPlayer = conn.profile.ID
		session.hintValue = hint
		session.hintValid = true
	}
	session.mutex.Unlock()
	if broadcast != nil {
		server.broadcastGame(broadcast)
		server.broadcastGamesChanged()
	}
	conn.send(TypeHint, envelope.RequestID, hint)
}

func (server *Server) sendRuleError(conn *client, requestID string, err error) {
	var ruleError *game.RuleError
	if errors.As(err, &ruleError) {
		conn.send(TypeError, requestID, ErrorPayload{Code: ruleError.Code, Message: ruleError.Message})
		return
	}
	conn.send(TypeError, requestID, ErrorPayload{Code: "invalid_move", Message: err.Error()})
}
