// Windows whose next content context menu belongs to a finished wheel gesture.
const suppressMenu = new WeakSet();
const guarded = new WeakSet();

export class WheelTabParent extends JSWindowActorParent {
  receiveMessage({ name, data }) {
    const win = this.browsingContext.topChromeWindow;
    if (!win?.gBrowser) {
      return;
    }
    if (name === "WheelTab:RightDown") {
      suppressMenu.delete(win);
    } else if (name === "WheelTab:Switch") {
      win.gBrowser.tabContainer.advanceSelectedTab(data.dir, true);
      suppressMenu.add(win);
      guardContextMenu(win);
    }
  }
}

// The button is released over the newly selected tab, whose document never saw the
// gesture, so its context menu has to be cancelled from the chrome side.
function guardContextMenu(win) {
  if (guarded.has(win)) {
    return;
  }
  guarded.add(win);
  win.document.getElementById("contentAreaContextMenu").addEventListener("popupshowing", event => {
    if (event.target.id === "contentAreaContextMenu" && suppressMenu.has(win)) {
      suppressMenu.delete(win);
      event.preventDefault();
    }
  });
}
