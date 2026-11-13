## Why

The overview shows each deployment's folder, but people want to open it to look at its files, and browsers refuse to follow a `file://` link from a page served over HTTP. The start page also needs refinement: the folder paths crowd the tiles, the page looks empty while the launcher takes seconds to list deployments, and the logo should lead back to it.

## What Changes

- Show the deployment folder on the overview as a link with a `file://` address.
- Opening that link asks the server to open the folder in the system's file manager: Finder on macOS, the desktop's file manager on Linux, Explorer on Windows.
- Add `POST /api/v1/deployments/{name}/open-folder`, which opens the folder of a listed deployment only.
- Drop the folder path from the start page's tiles, and show a progress indicator while the deployments are looked up.
- Make the Exasol logo a link to the start page on every page.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `deployment-overview`: the API can open a deployment's folder, the overview shows the folder as a link that opens it, and the tiles drop the folder and show progress while loading.
- `app-shell`: the Exasol logo links to the Deployments page.

## Impact

- New HTTP endpoint behind the existing session and origin checks.
- The server runs the platform's opener (`open`, `xdg-open`, `explorer`) on the deployment directory the launcher lists.
- The design canvas already shows the folder links.
