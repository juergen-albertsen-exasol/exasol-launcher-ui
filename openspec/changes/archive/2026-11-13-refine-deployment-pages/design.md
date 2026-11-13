## Context

The tiles and the overview show the deployment directory as text. A page served
from `http://127.0.0.1` cannot navigate to `file://` URLs: browsers block that
navigation. The server, however, runs on the same host as the browser.

## Goals / Non-Goals

**Goals:** open a deployment's folder from the UI on macOS, Linux, and Windows;
keep the `file://` address available for copying.

**Non-Goals:** browsing files inside the UI; opening paths other than listed
deployment directories.

## Decisions

**The server opens the folder.** A click calls
`POST /api/v1/deployments/{name}/open-folder`; the server looks the name up in
the launcher listing and runs the platform's opener on that path: `open` on
macOS, `xdg-open` on Linux, `explorer` on Windows (its exit status is ignored,
because Explorer returns 1 even on success). The browser never sends a path, so
the endpoint cannot open anything but listed deployment directories, and the
existing session and origin checks keep other pages from calling it.
Alternative: a plain `file://` link, rejected because browsers block it.

**The link keeps its `file://` address.** The anchor's `href` is the folder's
`file://` URL, so copying the link works; the click handler prevents the
navigation and calls the endpoint instead.

**Tiles stay single links.** Without the folder, a tile is again one link to the
overview. The progress indicator is the Material spinner the Administration UI
uses, shown while the listing has neither arrived nor failed.

**One opener for folders and the access URL.** The existing `browser.Open`
gains the Windows case and serves both the `--open` flag and folders.

## Risks / Trade-offs

- [No file manager on a headless Linux host] → The endpoint reports the
  opener's error as a problem response; the page shows it.
