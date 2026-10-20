# Words with Learners

A self-hosted, turn-based word game for language learners. The interface is in English, while the words on the board come from a selected foreign-language dictionary at a chosen CEFR level.

The game uses a Words With Friends style 15×15 board, a server-authoritative tile bag, and configurable rack sizes. Everything is served by one Go binary with an embedded static frontend.

## Features

- Languages: Dutch, English, German, Spanish, French.
- CEFR levels: A1, A2, B1, B2, C1, C2.
- 2–4 players per game.
- Configurable maximum word length and rack size (5–15).
- Server-side move validation, scoring, hints, bag/rack management, and replay checks.
- Pass, exchange, blanks, bingo bonuses, and end-game scoring.
- Searchable player directory reused by the invite dialog.
- Pannable, zoomable, grid-aligned board up to 2×.
- Local SVG flags and flag-inspired bonus-square themes.
- JSON profiles and games stored in a configurable data directory.

## Requirements

- Go 1.26 or newer.
- No frontend build step, Node.js, database, or external assets.

## Run

```sh
go run ./cmd/server
```

Open <http://localhost:8080>. Register an account, create a game, and invite another registered player.

Useful flags:

```text
-addr :8080                 HTTP listen address
-data ~/.config/words-with-learners
                            profile and game data directory
-dev                        serve frontend and dictionaries from disk
-public public              frontend directory used with -dev
-dictionaries dictionaries  dictionary directory used with -dev
-verify-games               replay every stored game on startup
```

For development:

```sh
go run ./cmd/server -dev -addr :8080
```

## Gameplay

- Drag tiles from your rack onto the board.
- The first word must cover the center square.
- Every formed word must exist in the selected language and CEFR dictionary.
- `?` shows an English hint for a playable word.
- `Recall` returns tentative board tiles to your rack.
- `Submit` sends the move to the server. The server recalculates and validates everything.
- `Pass` skips your turn. `Exchange` swaps selected rack tiles.
- The game ends when a rack is emptied with an empty bag, or after three scoreless turns.

## Dictionaries

Each language has an `alphabet.json` with tile values and counts, plus one word file per CEFR level:

```text
dictionaries/nl/alphabet.json
dictionaries/nl/a1.json
dictionaries/nl/a2.json
...
```

Word files use this format:

```json
{
  "schemaVersion": 1,
  "language": "nl",
  "level": "A1",
  "words": [
    { "word": "HUIS", "hint": "house" }
  ]
}
```

The bundled dictionaries are small curated demo sets, not complete competitive word lists. Import larger lists with:

```sh
go run ./cmd/dictimport \
  -input words.csv \
  -language nl \
  -level B1 \
  -dictionaries dictionaries
```

`words.csv` can contain `word,hint` rows. JSON input with a `words` array is also accepted. Words using letters missing from the alphabet are skipped with a warning.

## Data and security

- Data is stored as JSON files under `data/profiles` and `data/games`.
- Writes are atomic (temporary file, sync, rename).
- Passwords use bcrypt hashes.
- Sessions use HttpOnly, SameSite=Lax cookies. Only hashed tokens are stored in `data/sessions.json`, so logins survive server restarts.
- Tiles, bag order, scores, and dictionary checks are server-side. Clients only send tile IDs and coordinates.
- Move IDs are idempotent and `revision` values reject stale moves.
- Game files include an alphabet snapshot so `-verify-games` can replay and verify stored games.

## Tests

```sh
go test ./...
go vet ./...
```

The test suite covers board layout, scoring, move validation, replay determinism, idempotent moves, and a full WebSocket game flow including hidden racks and stale-move rejection.

## Layout

```text
cmd/server          HTTP + WebSocket server
cmd/dictimport      dictionary importer
internal/game       board, rules, scoring, bag, replay
internal/dictionary loader and lookup
internal/server     protocol, WebSocket hub, hint search
internal/store      atomic JSON storage
internal/auth       bcrypt auth and sessions
public              static HTML/CSS/ES modules (embedded)
dictionaries        demo alphabets and word lists (embedded)
```

## Current scope

No chat, timers, spectators, matchmaking, or full commercial dictionaries. The static frontend avoids a build step so it can later be packaged into a WebView.
