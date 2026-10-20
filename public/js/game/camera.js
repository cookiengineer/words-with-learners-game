const ZOOM_LEVELS = [0.5, 0.6, 0.7, 0.8, 0.9, 1, 1.2, 1.4, 1.6, 1.8, 2];

export class Camera {
  constructor(viewport, board, tileSize = 44) {
    this.viewport = viewport;
    this.board = board;
    this.tileSize = tileSize;
    this.zoom = 1;
    this.x = 0;
    this.y = 0;
    this.panning = false;
    this.panStartX = 0;
    this.panStartY = 0;
    this.panOriginX = 0;
    this.panOriginY = 0;
  }

  viewportSize() {
    const rect = this.viewport.getBoundingClientRect();
    return { width: rect.width, height: rect.height };
  }

  boardSize() {
    return this.tileSize * 15 * this.zoom;
  }

  fit(instant = false) {
    const { width, height } = this.viewportSize();
    if (width === 0 || height === 0) {
      return;
    }
    const fit = Math.min(width / (this.tileSize * 15), height / (this.tileSize * 15));
    let zoom = ZOOM_LEVELS[0];
    for (const level of ZOOM_LEVELS) {
      if (level <= fit) {
        zoom = level;
      }
    }
    this.zoom = zoom;
    const board = this.boardSize();
    this.x = (width - board) / 2;
    this.y = (height - board) / 2;
    this.snap();
    this.apply(instant);
  }

  setZoom(zoom, clientX, clientY) {
    const { width, height } = this.viewportSize();
    const rect = this.viewport.getBoundingClientRect();
    const anchorX = clientX === undefined ? width / 2 : clientX - rect.left;
    const anchorY = clientY === undefined ? height / 2 : clientY - rect.top;
    const boardX = (anchorX - this.x) / this.zoom;
    const boardY = (anchorY - this.y) / this.zoom;
    this.zoom = Math.max(ZOOM_LEVELS[0], Math.min(ZOOM_LEVELS[ZOOM_LEVELS.length - 1], zoom));
    this.x = anchorX - boardX * this.zoom;
    this.y = anchorY - boardY * this.zoom;
    this.clamp();
    this.snap();
    this.apply();
  }

  zoomIn(clientX, clientY) {
    const index = ZOOM_LEVELS.findIndex((level) => level > this.zoom + 0.001);
    this.setZoom(ZOOM_LEVELS[index < 0 ? ZOOM_LEVELS.length - 1 : index], clientX, clientY);
  }

  zoomOut(clientX, clientY) {
    const index = [...ZOOM_LEVELS].reverse().findIndex((level) => level < this.zoom - 0.001);
    const levels = [...ZOOM_LEVELS].reverse();
    this.setZoom(levels[index < 0 ? levels.length - 1 : index], clientX, clientY);
  }

  startPan(clientX, clientY) {
    this.panning = true;
    this.panStartX = clientX;
    this.panStartY = clientY;
    this.panOriginX = this.x;
    this.panOriginY = this.y;
    this.viewport.classList.add("panning");
  }

  movePan(clientX, clientY) {
    if (!this.panning) {
      return;
    }
    this.x = this.panOriginX + clientX - this.panStartX;
    this.y = this.panOriginY + clientY - this.panStartY;
    this.clamp();
    this.apply();
  }

  endPan() {
    this.panning = false;
    this.viewport.classList.remove("panning");
    this.snap();
    this.apply();
  }

  snap() {
    const step = this.tileSize * this.zoom;
    this.x = Math.round(this.x / step) * step;
    this.y = Math.round(this.y / step) * step;
    this.clamp();
  }

  clamp() {
    const { width, height } = this.viewportSize();
    const board = this.boardSize();
    const margin = this.tileSize * this.zoom;
    if (board <= width) {
      this.x = (width - board) / 2;
    } else {
      this.x = Math.max(width - board - margin, Math.min(margin, this.x));
    }
    if (board <= height) {
      this.y = (height - board) / 2;
    } else {
      this.y = Math.max(height - board - margin, Math.min(margin, this.y));
    }
  }

  apply(instant = false) {
    this.board.style.transform = `translate(${this.x}px, ${this.y}px) scale(${this.zoom})`;
    if (instant) {
      this.board.classList.add("no-transition");
      requestAnimationFrame(() => this.board.classList.remove("no-transition"));
    }
  }

  cellFromClient(clientX, clientY) {
    const rect = this.board.getBoundingClientRect();
    if (clientX < rect.left || clientY < rect.top || clientX >= rect.right || clientY >= rect.bottom) {
      return null;
    }
    const size = rect.width / 15;
    const x = Math.floor((clientX - rect.left) / size);
    const y = Math.floor((clientY - rect.top) / size);
    if (x < 0 || x >= 15 || y < 0 || y >= 15) {
      return null;
    }
    return { x, y };
  }

  resize() {
    this.clamp();
    this.snap();
    this.apply(true);
  }
}
