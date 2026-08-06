# People and Events

People, places, and events are auto-generated views from faces, GPS, and capture time — useful for discovering content. **Editing them changes organization and display, never the original media files.**

| Entry | Question it answers | Based on |
| --- | --- | --- |
| **People** | Which media show the same person? | Face detection and clustering |
| **Places** | Where were the photos taken? | GPS and clustering |
| **Events** | Which media belong to the same time span? | Time, location, algorithm |

> Newly imported media have no results yet — thumbnails, faces, and place clustering may still be queued. Check the [Server Monitor](./monitor) before deciding something is wrong.

## People

1. Open **Collections → People**; switch between **shown** and **all** (hiding only removes from the default grid).
2. Open a card for the detail view: the upper half is a media browser.
3. Click **Edit**:
   - **Info**: rename (marks as confirmed), hide;
   - **Faces**: set cover, move or remove faces in batch;
   - **Merge**: merge other people into this one (inspect both faces first).

> Moving/removing faces only changes the association; media is not modified and not trashed.

## Places

1. Open **Collections → Places**, choose the scope;
2. Pan and zoom the map; dense areas show numbered markers, click to zoom in;
3. Click a marker for a preview, then open the viewer.

The map only shows media with GPS; media without GPS are never assigned a location. Trip cards are computed review views, not editable albums.

## Events

1. Open **Collections → Events**; cards show auto-generated name, time range, cover, and count.
2. Inside an event you can: rename, set cover, add/remove media, split before selected media, move to another event, merge, hide, share.
3. If the list is empty or stale, click **Rebuild** in the title bar.

> Add/remove/split/merge writes manual constraints; “waiting for event rebuild” means the constraints are saved but regrouping is not published yet — re-check after rebuilding. Removing media from an event does not delete it; to delete, use **Move to trash** in the library.

## When results are missing

1. Check the [Server Monitor](./monitor) for failing tasks;
2. Wrong people groups: **Rebuild person recognition** at the top of the **Manage** page (cross-repository);
3. Missing map points: **Rebuild location clusters** in the repository card **⋯** menu;
4. Stale/empty events: **Collections → Events → Rebuild**;
5. Confirm the library scope is not set to another repository.

::: info Privacy reminder
People, locations, and capture times can be sensitive. Check the people and places scope before creating a public share, and verify with a logged-out window what recipients actually see.
:::
