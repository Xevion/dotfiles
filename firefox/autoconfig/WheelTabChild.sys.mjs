/** Holding the right button while scrolling switches tabs instead of scrolling the page. */

// Pixel-mode wheels (touchpads, Windows precision mice) emit many small deltas per gesture.
const PIXELS_PER_SWITCH = 50;

export class WheelTabChild extends JSWindowActorChild {
  #switched = false;
  #pixels = 0;

  handleEvent(event) {
    switch (event.type) {
      case "mousedown":
        if (event.button === 2) {
          this.#switched = false;
          this.#pixels = 0;
          this.sendAsyncMessage("WheelTab:RightDown");
        }
        break;
      case "wheel":
        this.#onWheel(event);
        break;
      case "contextmenu":
        if (this.#switched) {
          this.#switched = false;
          event.preventDefault();
          event.stopImmediatePropagation();
        }
        break;
    }
  }

  #onWheel(event) {
    // `buttons` reflects the live button state, so this keeps working after the
    // switch puts a different document (with its own actor) under the cursor.
    if (!(event.buttons & 2) || event.deltaY === 0) {
      return;
    }
    event.preventDefault();
    event.stopImmediatePropagation();
    this.#switched = true;

    let dir = Math.sign(event.deltaY);
    if (event.deltaMode === event.DOM_DELTA_PIXEL) {
      this.#pixels += event.deltaY;
      if (Math.abs(this.#pixels) < PIXELS_PER_SWITCH) {
        return;
      }
      dir = Math.sign(this.#pixels);
      this.#pixels = 0;
    }
    this.sendAsyncMessage("WheelTab:Switch", { dir });
  }
}
