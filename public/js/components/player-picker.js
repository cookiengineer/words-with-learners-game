import { socket } from "../socket.js";
import { el, toast } from "../ui.js";

class PlayerPicker extends HTMLElement {
  connectedCallback() {
    if (this.initialized) {
      return;
    }
    this.initialized = true;
    this.max = Number(this.getAttribute("max") || 1);
    this.selected = [];
    this.results = [];
    this.nextOffset = 0;
    this.searchTimer = null;
    this.requestCounter = 0;
    this.render();
    this.input = this.querySelector("input[type='search']");
    this.resultsNode = this.querySelector(".player-results");
    this.input.addEventListener("input", () => {
      clearTimeout(this.searchTimer);
      this.searchTimer = setTimeout(() => this.search(0, false), 180);
    });
    this.search(0, false);
  }

  disconnectedCallback() {
    clearTimeout(this.searchTimer);
  }

  render() {
    this.replaceChildren(el("label", { text: "Find a player" }, [
      el("input", {
        type: "search",
        placeholder: this.getAttribute("placeholder") || "Search players",
        autocomplete: "off",
      }),
    ]), el("div", { class: "player-results", role: "listbox" }));
  }

  setMax(value) {
    this.max = Math.max(1, Number(value) || 1);
    if (this.selected.length > this.max) {
      this.selected = this.selected.slice(0, this.max);
      this.emit();
    }
    this.updateRows();
  }

  setSelected(profiles) {
    const seen = new Set();
    this.selected = [];
    for (const profile of profiles) {
      if (seen.has(profile.id) || this.selected.length >= this.max) {
        continue;
      }
      seen.add(profile.id);
      this.selected.push({
        id: profile.id,
        username: profile.username,
        displayName: profile.displayName,
      });
    }
    this.updateRows();
    this.emit();
  }

  remove(id) {
    this.selected = this.selected.filter((profile) => profile.id !== id);
    this.updateRows();
    this.emit();
  }

  toggle(profile) {
    const exists = this.selected.some((entry) => entry.id === profile.id);
    if (exists) {
      this.remove(profile.id);
      return;
    }
    if (this.max === 1) {
      this.selected = [profile];
    } else if (this.selected.length < this.max) {
      this.selected.push(profile);
    } else {
      toast(`You can select at most ${this.max} players`);
      return;
    }
    this.updateRows();
    this.emit();
  }

  emit() {
    this.dispatchEvent(new CustomEvent("selectionchange", {
      detail: { selected: this.selected.map((profile) => ({ ...profile })) },
    }));
  }

  async search(offset, append) {
    const request = ++this.requestCounter;
    const query = this.input?.value?.trim() || "";
    try {
      const payload = await socket.request("players.search", { query, offset, limit: 30 });
      if (request !== this.requestCounter) {
        return;
      }
      const players = payload?.players || [];
      this.results = append ? [...this.results, ...players] : players;
      this.nextOffset = payload?.nextOffset || 0;
      this.renderResults();
    } catch (error) {
      if (request === this.requestCounter) {
        this.resultsNode.replaceChildren(el("p", { class: "empty", text: error.message }));
      }
    }
  }

  renderResults() {
    const selected = new Set(this.selected.map((profile) => profile.id));
    const nodes = [];
    if (this.results.length === 0) {
      nodes.push(el("p", { class: "empty", text: "No players found." }));
    }
    for (const profile of this.results) {
      const isSelected = selected.has(profile.id);
      nodes.push(el("button", {
        type: "button",
        class: "player-result",
        dataset: { selected: String(isSelected), id: profile.id },
        role: "option",
        "aria-selected": String(isSelected),
        onclick: () => this.toggle(profile),
      }, [
        el("span", {}, [
          el("strong", { text: profile.displayName }),
          el("br"),
          el("small", { text: `@${profile.username}` }),
        ]),
        el("span", { text: isSelected ? "Selected" : "Select" }),
      ]));
    }
    if (this.nextOffset > 0) {
      nodes.push(el("button", {
        type: "button",
        class: "player-more",
        text: "Load more",
        onclick: () => this.search(this.nextOffset, true),
      }));
    }
    this.resultsNode.replaceChildren(...nodes);
  }

  updateRows() {
    if (this.resultsNode) {
      this.renderResults();
    }
  }
}

customElements.define("player-picker", PlayerPicker);
