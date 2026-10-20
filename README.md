# Words with Learners

A self-hosted, turn-based word game for language learners. The interface is in English, while hints and gameplay are centred on a selected foreign-language CEFR level.

The game uses a Words With Friends style 15×15 board, a server-authoritative tile bag, and configurable rack sizes. Everything is served by one Go binary with an embedded static frontend.

## Features

- Languages: Dutch, English, German, Spanish, French, Italian, Russian.
- CEFR levels: A1, A2, B1, B2, C1, C2.
- 1–4 players per game; single-player games are local training games.
- Configurable maximum word length and rack size (5–15).
- Server-side move validation, scoring, hints, bag/rack management, and replay checks.
- Automatic rack reshuffling when the current player has no playable word and the bag has tiles.
- Pass, blanks, bingo bonuses, and end-game scoring.
- Searchable player directory reused by the invite dialog.
- Game creators can stop and delete their games from the overview.
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

Open <http://localhost:8080>. Enter a username and password; a new account is created automatically if the username is new. Then create a game and invite another player, or choose one player to train on your own.

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
- Every formed word must exist in the selected language, at any CEFR level.
- `?` shows an English hint for a playable word.
- `Recall` returns tentative board tiles to your rack.
- `Submit` sends the move to the server. The server recalculates and validates everything.
- `Pass` skips your turn. There is no manual tile exchange; stuck racks are reshuffled automatically.
- If your rack has no legal word when your turn starts, the server automatically reshuffles it with bag tiles and notifies you.
- The game ends when a rack is emptied with an empty bag, or after three scoreless turns.

## Dictionaries

Each language has an `alphabet.json` with tile values and counts, one word file per CEFR level, and two curated supplement files:

```text
dictionaries/nl/alphabet.json
dictionaries/nl/a1.json
dictionaries/nl/a2.json
...
dictionaries/nl/cities.json
dictionaries/nl/sights.json
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

City and sight supplements use the same word format with a `kind` field:

```json
{
  "schemaVersion": 1,
  "language": "nl",
  "kind": "cities",
  "words": [
    { "word": "ROTTERDAM", "hint": "major port city in South Holland" }
  ]
}
```

The bundled dictionaries for Dutch, English, German, Spanish, French, Italian, and Russian are generated from the Lingvoberry A0–C2 word lists. A0 is merged into A1.

The city and sight supplements cover the target countries: the Netherlands, the United Kingdom, Germany, Spain, France, Italy, and Russia. They contain localized single-token names with English hints. Names that require spaces or hyphens are omitted because the board only accepts single words.

Hints, suggestions, and automatic reshuffles use words from the selected level plus the city and sight supplements. Move validation accepts any legitimate word from any level of that language, so an A1 game can use a C2 word or a city name if the player finds one.

Hints are taken from the English definitions for each word's language section in English Wiktionary. The importer reads a ZIM file through the external `zimdump` and `zimsearch` commands, so the server itself does not need the ZIM at runtime.

Regenerate all bundled dictionaries with:

```sh
go run ./cmd/dictimport \
  -input ./path/to/wordlists \
  -zim ./path/to/wiktionary_en_all_nopic_2026-08.zim \
  -workers 16 \
  -report /tmp/dictionary-report.csv
```

Single files can still be imported with:

```sh
go run ./cmd/dictimport \
  -input words.csv \
  -language nl \
  -level B1 \
  -dictionaries dictionaries
```

CSV input accepts both `word,hint` rows and the Lingvoberry `rank,word,cefr` format. JSON input with a `words` array is also accepted. Words with unknown letters, punctuation, or a length outside 2–15 are skipped and listed in the report. Words without a Wiktionary definition remain playable and use the game's generic length hint.

The word lists are from [Lingvoberry](https://lingvoberry.com/) and are licensed under CC BY-SA 4.0. Hints are derived from English Wiktionary content, licensed under CC BY-SA.

## Data and security

- Data is stored as JSON files under `data/profiles` and `data/games`.
- Writes are atomic (temporary file, sync, rename).
- Passwords use bcrypt hashes. Logging in with a new username creates its account automatically; existing usernames require the matching password.
- Sessions use HttpOnly, SameSite=Lax cookies. Only hashed tokens are stored in `data/sessions.json`, so logins survive server restarts.
- Tiles, bag order, scores, and dictionary checks are server-side. Clients only send tile IDs and coordinates.
- Move IDs are idempotent and `revision` values reject stale moves.
- Game files include an alphabet snapshot so `-verify-games` can replay and verify stored games, including recorded automatic reshuffles.

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
internal/wiktionary Wiktionary ZIM lookup and hint extraction
internal/server     protocol, WebSocket hub, hint search
internal/store      atomic JSON storage
internal/auth       bcrypt auth and sessions
public              static HTML/CSS/ES modules (embedded)
dictionaries        alphabets and CEFR word lists (embedded)
```

## Current scope

No chat, timers, spectators, matchmaking, or full commercial dictionaries. The static frontend avoids a build step so it can later be packaged into a WebView.
