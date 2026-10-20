import { socket } from "../socket.js";
import { Camera } from "../game/camera.js";
import { clear, clearToasts, el, flagFor, languageNames, template, toast, uniqueID } from "../ui.js";

const BONUS_LABELS = {
  tw: "3W",
  dw: "2W",
  tl: "3L",
  dl: "2L",
  start: "★",
};

export class GameView {
  constructor(container, context, gameId) {
    this.container = container;
    this.context = context;
    this.gameId = gameId;
    this.state = null;
    this.pending = [];
    this.cells = new Map();
    this.previewTimer = null;
    this.previewSequence = 0;
    this.lastPreviewError = "";
    this.lastReshuffleNotice = "";
    this.lastTurnKey = "";
    this.destroyed = false;
    this.dragCleanup = null;
    this.blankResolver = null;
    this.resizeObserver = null;
    this.lastViewportWidth = null;
  }

  mount() {
    clear(this.container);
    clearToasts();
    this.container.append(template("game-template"));
    this.flag = this.container.querySelector("#game-flag");
    this.language = this.container.querySelector("#game-language");
    this.level = this.container.querySelector("#game-level");
    this.scoreboard = this.container.querySelector("#scoreboard");
    this.viewport = this.container.querySelector("#board-viewport");
    this.board = this.container.querySelector("#board");
    this.rack = this.container.querySelector("#rack");
    this.preview = this.container.querySelector("#preview");
    this.recall = this.container.querySelector("#recall");
    this.passButton = this.container.querySelector("#pass");
    this.submitButton = this.container.querySelector("#submit");
    this.hintButton = this.container.querySelector("#hint");
    this.hintDialog = document.getElementById("hint-dialog");
    this.hintText = document.getElementById("hint-text");
    this.hintMeta = document.getElementById("hint-meta");
    this.blankDialog = document.getElementById("blank-dialog");
    this.blankForm = document.getElementById("blank-form");
    this.blankInput = document.getElementById("blank-letter");
    this.buildBoard();
    this.camera = new Camera(this.viewport, this.board);
    this.bind();
    this.offState = socket.on("game.state", ({ payload }) => {
      if (payload && payload.id === this.gameId) {
        this.update(payload);
      }
    });
    this.offConnected = socket.on("connected", () => {
      this.requestState();
    });
    this.resizeObserver = new ResizeObserver(() => {
      const { width } = this.camera.viewportSize();
      const roundedWidth = Math.round(width);
      if (roundedWidth !== this.lastViewportWidth) {
        this.lastViewportWidth = roundedWidth;
        this.camera.fit(true);
      } else {
        this.camera.resize();
      }
    });
    this.resizeObserver.observe(this.viewport);
    requestAnimationFrame(() => this.camera.fit(true));
    this.requestState();
    return () => this.destroy();
  }

  buildBoard() {
    clear(this.board);
    this.cells.clear();
    for (let y = 0; y < 15; y += 1) {
      for (let x = 0; x < 15; x += 1) {
        const cell = el("div", { class: "cell", dataset: { x: String(x), y: String(y) } });
        const key = `${x},${y}`;
        this.cells.set(key, cell);
        this.board.append(cell);
      }
    }
  }

  bind() {
    this.container.querySelector("#leave-game").addEventListener("click", () => this.context.onLeave());
    this.container.querySelector("#zoom-in").addEventListener("click", () => this.camera.zoomIn());
    this.container.querySelector("#zoom-out").addEventListener("click", () => this.camera.zoomOut());
    this.container.querySelector("#zoom-fit").addEventListener("click", () => this.camera.fit());
    this.viewport.addEventListener("pointerdown", (event) => this.onViewportPointerDown(event));
    this.viewport.addEventListener("pointermove", (event) => {
      if (this.camera.panning) {
        this.camera.movePan(event.clientX, event.clientY);
      }
    });
    const endPan = () => this.camera.endPan();
    this.viewport.addEventListener("pointerup", endPan);
    this.viewport.addEventListener("pointercancel", endPan);
    this.viewport.addEventListener("wheel", (event) => {
      event.preventDefault();
      if (event.deltaY < 0) {
        this.camera.zoomIn(event.clientX, event.clientY);
      } else {
        this.camera.zoomOut(event.clientX, event.clientY);
      }
    }, { passive: false });
    this.recall.addEventListener("click", () => {
      this.pending = [];
      this.render();
    });
    this.submitButton.addEventListener("click", () => this.submit());
    this.passButton.addEventListener("click", () => this.pass());
    this.hintButton.addEventListener("click", () => this.showHint());
    for (const button of this.hintDialog.querySelectorAll("[data-close-hint]")) {
      button.onclick = () => this.hintDialog.close();
    }
    this.blankForm.onsubmit = (event) => {
      event.preventDefault();
      const letter = this.blankInput.value.trim().toUpperCase();
      if (!letter) {
        return;
      }
      this.resolveBlank(letter);
    };
    document.getElementById("blank-cancel").onclick = () => this.resolveBlank(null);
    this.blankDialog.oncancel = (event) => {
      event.preventDefault();
      this.resolveBlank(null);
    };
  }

  onViewportPointerDown(event) {
    if (event.button !== undefined && event.button !== 0) {
      return;
    }
    const pendingNode = event.target.closest(".placed[data-pending='true']");
    if (pendingNode) {
      event.preventDefault();
      event.stopPropagation();
      const entry = this.pending.find((item) => item.tile.id === pendingNode.dataset.tileId);
      if (entry) {
        this.startTileDrag(entry.tile, event, entry);
      }
      return;
    }
    if (!this.state || this.state.status === "finished") {
      return;
    }
    this.camera.startPan(event.clientX, event.clientY);
    try {
      this.viewport.setPointerCapture(event.pointerId);
    } catch {
      return;
    }
  }

  async requestState() {
    try {
      const state = await socket.request("game.state", { gameId: this.gameId });
      if (!this.destroyed) {
        this.update(state);
      }
    } catch (error) {
      if (this.destroyed) {
        return;
      }
      if (error.code === "not_found" || error.code === "not_a_player") {
        toast(error.message);
        this.context.onLeave();
      } else {
        toast(error.message);
      }
    }
  }

  canPlay() {
    return this.state && this.state.status === "playing" && this.state.isYourTurn;
  }

  update(state) {
    if (this.destroyed || !state || state.id !== this.gameId) {
      return;
    }
    const previousRevision = this.state?.revision;
    this.state = state;
    if (!state.isYourTurn || state.status !== "playing") {
      if (this.pending.length > 0) {
        this.pending = [];
      }
    } else if (previousRevision !== state.revision) {
      const rackIds = new Set(state.rack.map((tile) => tile.id));
      this.pending = this.pending.filter((entry) => rackIds.has(entry.tile.id));
    }
    this.flag.src = flagFor(state.config.language);
    this.flag.alt = `${languageNames[state.config.language]} flag`;
    this.language.textContent = languageNames[state.config.language] || state.config.language;
    this.level.textContent = `${state.config.level} · ${state.config.maxWordLength} tiles · bag ${state.bagCount}`;
    this.board.dataset.language = state.config.language;
    this.renderScoreboard();
    this.render();
    this.notifyTurn();
    this.notifyReshuffle();
  }

  notifyTurn() {
    const key = `${this.state.status}:${this.state.currentPlayerId}`;
    if (key === this.lastTurnKey) {
      return;
    }
    this.lastTurnKey = key;
    if (this.state.status === "finished") {
      const winner = this.state.players.find((player) => player.id === this.state.winner);
      toast(winner ? `${winner.name} wins!` : "The game ended in a tie.", "turn");
      return;
    }
    const current = this.state.players.find((player) => player.id === this.state.currentPlayerId);
    if (this.state.isYourTurn) {
      toast("Your turn. Drag tiles onto the board, then submit.", "turn");
    } else {
      toast(`Waiting for ${current?.name || "another player"}.`, "turn");
    }
  }

  notifyReshuffle() {
    const reshuffle = this.state.autoReshuffle;
    if (!reshuffle || !this.state.isYourTurn) {
      return;
    }
    const you = this.state.players.find((player) => player.isYou);
    if (!you || reshuffle.playerId !== you.id || reshuffle.revision !== this.state.revision) {
      return;
    }
    const key = `${this.gameId}:${reshuffle.revision}:${reshuffle.playerId}`;
    if (this.lastReshuffleNotice === key) {
      return;
    }
    this.lastReshuffleNotice = key;
    toast("No playable word was available, so your tiles were reshuffled.", "reshuffle");
  }

  renderScoreboard() {
    const nodes = this.state.players.map((player) => el("li", {
      dataset: { active: String(player.isTurn), you: String(player.isYou) },
      title: player.online ? "Online" : "Offline",
    }, [
      el("span", { class: player.online ? "online" : "offline" }),
      el("span", { text: player.name }),
      el("strong", { text: String(player.score) }),
    ]));
    this.scoreboard.replaceChildren(...nodes);
  }

  render() {
    if (!this.state) {
      return;
    }
    this.renderBoard();
    this.renderRack();
    this.updateActions();
    this.schedulePreview();
  }

  renderBoard() {
    for (const cell of this.cells.values()) {
      cell.querySelector(".placed")?.remove();
      const bonus = this.state.bonuses?.[Number(cell.dataset.y)]?.[Number(cell.dataset.x)] || "none";
      if (cell.dataset.bonus !== bonus) {
        cell.dataset.bonus = bonus;
        cell.textContent = BONUS_LABELS[bonus] || "";
      }
    }
    const placedTiles = this.state.board.map((tile) => ({ ...tile, pending: false }));
    for (const entry of this.pending) {
      placedTiles.push({
        x: entry.x,
        y: entry.y,
        letter: entry.blankLetter || entry.tile.letter || "?",
        value: entry.tile.value,
        blank: entry.tile.blank,
        pending: true,
        tileId: entry.tile.id,
      });
    }
    for (const tile of placedTiles) {
      const cell = this.cells.get(`${tile.x},${tile.y}`);
      if (!cell) {
        continue;
      }
      const node = el("div", {
        class: "placed",
        dataset: { pending: String(Boolean(tile.pending)), tileId: tile.tileId || "" },
        text: tile.letter || "?",
      });
      if (tile.value > 0) {
        node.append(el("small", { text: String(tile.value) }));
      }
      if (tile.blank) {
        node.title = "Blank tile";
      }
      cell.append(node);
    }
  }

  renderRack() {
    const used = new Set(this.pending.map((entry) => entry.tile.id));
    const nodes = this.state.rack.map((tile) => {
      const node = el("button", {
        type: "button",
        class: "rack-tile",
        dataset: {
          used: String(used.has(tile.id)),
          blank: String(tile.blank),
        },
        disabled: !this.canPlay(),
      }, [tile.blank ? "?" : tile.letter, tile.value > 0 ? el("small", { text: String(tile.value) }) : null]);
      node.addEventListener("pointerdown", (event) => {
        if (this.canPlay() && !used.has(tile.id)) {
          this.startTileDrag(tile, event, null);
        }
      });
      return node;
    });
    this.rack.replaceChildren(...nodes);
  }

  updateActions() {
    const canPlay = this.canPlay();
    this.recall.disabled = !canPlay || this.pending.length === 0;
    this.submitButton.disabled = !canPlay || this.pending.length === 0;
    this.passButton.disabled = !canPlay;
    this.hintButton.disabled = !canPlay;
  }

  occupied(x, y) {
    if (this.state.board.some((tile) => tile.x === x && tile.y === y)) {
      return true;
    }
    return this.pending.some((entry) => entry.x === x && entry.y === y);
  }

  startTileDrag(tile, event, existing) {
    if (!this.canPlay()) {
      return;
    }
    event.preventDefault();
    event.stopPropagation();
    if (this.dragCleanup) {
      this.dragCleanup();
    }
    const ghost = el("div", { class: "tile ghost", text: tile.blank ? "?" : tile.letter });
    if (tile.value > 0) {
      ghost.append(el("small", { text: String(tile.value) }));
    }
    document.body.append(ghost);
    ghost.style.left = `${event.clientX}px`;
    ghost.style.top = `${event.clientY}px`;
    if (existing) {
      this.pending = this.pending.filter((entry) => entry !== existing);
      this.render();
    }
    let lastX = event.clientX;
    let lastY = event.clientY;
    const move = (moveEvent) => {
      lastX = moveEvent.clientX;
      lastY = moveEvent.clientY;
      ghost.style.left = `${lastX}px`;
      ghost.style.top = `${lastY}px`;
    };
    const finish = async (upEvent) => {
      cleanup();
      const rect = this.viewport.getBoundingClientRect();
      const inside = upEvent.clientX >= rect.left && upEvent.clientX < rect.right && upEvent.clientY >= rect.top && upEvent.clientY < rect.bottom;
      const cell = inside ? this.camera.cellFromClient(upEvent.clientX, upEvent.clientY) : null;
      if (!cell || this.occupied(cell.x, cell.y)) {
        this.render();
        return;
      }
      let blankLetter = existing?.blankLetter || "";
      if (tile.blank && !blankLetter) {
        blankLetter = await this.askBlankLetter();
        if (!blankLetter) {
          this.render();
          return;
        }
      }
      this.pending.push({ tile, x: cell.x, y: cell.y, blankLetter });
      this.render();
    };
    const cleanup = () => {
      ghost.remove();
      window.removeEventListener("pointermove", move);
      window.removeEventListener("pointerup", finish);
      window.removeEventListener("pointercancel", cancel);
      this.dragCleanup = null;
    };
    const cancel = () => {
      cleanup();
      this.render();
    };
    this.dragCleanup = cleanup;
    window.addEventListener("pointermove", move);
    window.addEventListener("pointerup", finish, { once: true });
    window.addEventListener("pointercancel", cancel, { once: true });
  }

  askBlankLetter() {
    if (this.blankDialog.open) {
      return Promise.resolve(null);
    }
    this.blankInput.value = "";
    this.blankDialog.showModal();
    this.blankInput.focus();
    return new Promise((resolve) => {
      this.blankResolver = resolve;
    });
  }

  resolveBlank(letter) {
    const resolve = this.blankResolver;
    this.blankResolver = null;
    if (this.blankDialog.open) {
      this.blankDialog.close();
    }
    if (resolve) {
      resolve(letter ? letter.trim().toUpperCase().slice(0, 1) : null);
    }
  }

  movePayload(kind, extra = {}) {
    const placements = this.pending.map((entry) => {
      const placement = { tileId: entry.tile.id, x: entry.x, y: entry.y };
      if (entry.blankLetter) {
        placement.blankLetter = entry.blankLetter;
      }
      return placement;
    });
    return {
      moveId: uniqueID(),
      expectedRevision: this.state.revision,
      kind,
      ...(kind === "place" ? { placements } : {}),
      ...extra,
    };
  }

  schedulePreview() {
    clearTimeout(this.previewTimer);
    const sequence = ++this.previewSequence;
    if (!this.canPlay() || this.pending.length === 0) {
      this.preview.textContent = "";
      this.lastPreviewError = "";
      return;
    }
    this.previewTimer = setTimeout(async () => {
      try {
        const payload = await socket.request("move.preview", {
          gameId: this.gameId,
          move: { kind: "place", placements: this.movePayload("place").placements },
        });
        if (sequence !== this.previewSequence) {
          return;
        }
        this.preview.textContent = `${payload.score} pts${payload.bingo ? " · Bingo!" : ""}`;
        this.lastPreviewError = "";
      } catch (error) {
        if (sequence === this.previewSequence) {
          this.preview.textContent = "Not valid";
          if (error.code !== this.lastPreviewError) {
            this.lastPreviewError = error.code;
            toast(error.message, "preview");
          }
        }
      }
    }, 280);
  }

  async submit() {
    if (!this.canPlay() || this.pending.length === 0) {
      return;
    }
    this.submitButton.disabled = true;
    try {
      const payload = await socket.request("move.submit", {
        gameId: this.gameId,
        move: this.movePayload("place"),
      });
      this.pending = [];
      this.preview.textContent = "";
      this.render();
      toast(`Played for ${payload.move.score} points`);
      this.requestState();
    } catch (error) {
      toast(error.message);
      this.updateActions();
    }
  }

  async pass() {
    if (!this.canPlay() || !confirm("Pass your turn?")) {
      return;
    }
    try {
      await socket.request("move.submit", {
        gameId: this.gameId,
        move: this.movePayload("pass"),
      });
      this.pending = [];
      this.render();
      this.requestState();
    } catch (error) {
      toast(error.message);
    }
  }

  async showHint() {
    if (!this.canPlay()) {
      return;
    }
    try {
      const payload = await socket.request("hint.get", { gameId: this.gameId });
      if (payload.found) {
        this.hintText.textContent = payload.hint;
        this.hintMeta.textContent = `${payload.length} letters · best score ${payload.score}`;
      } else {
        this.hintText.textContent = "No playable word was found for your tiles on this board.";
        this.hintMeta.textContent = "";
      }
      this.hintDialog.showModal();
    } catch (error) {
      toast(error.message);
    }
  }

  destroy() {
    this.destroyed = true;
    clearToasts();
    clearTimeout(this.previewTimer);
    if (this.offState) {
      this.offState();
    }
    if (this.offConnected) {
      this.offConnected();
    }
    if (this.resizeObserver) {
      this.resizeObserver.disconnect();
    }
    if (this.dragCleanup) {
      this.dragCleanup();
    }
    if (this.blankDialog.open) {
      this.blankDialog.close();
    }
    if (this.hintDialog.open) {
      this.hintDialog.close();
    }
  }
}
