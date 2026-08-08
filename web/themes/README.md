# Themes

A theme is one CSS file, `<name>.css`, that redefines the design tokens under
`[data-theme="<name>"]`. The board asks for `/themes/<name>.css` whenever a
project's `projects.yaml` entry names a theme, so **adding one takes no code
change and no rebuild** — nothing in thingd or the frontend enumerates the names that exist.
An unknown name is a 404 and the board keeps the default palette.

The files in this directory are the built-in set, embedded in the `thingd`
binary. One more layer sits above them on disk: `themes/` under thingd's state
directory, beside `projects.yaml` — `$THING_DATA_DIR`, else
`$XDG_STATE_HOME/thingd`, else `~/.local/state/thingd`. Themes resolve through
exactly the rule `projects.yaml` does, so pointing `THING_DATA_DIR` somewhere
moves both and a single directory holds a complete thingd setup.

Both layers contribute when both define a name, concatenated built-in first, so
yours **overrides through the normal CSS cascade and only restates the tokens it
changes**. To warm up the built-in teal without redefining it:

```css
/* ~/.local/state/thingd/themes/teal.css */
[data-theme="teal"] {
  --amber: #00bcd4;
}
```

A name the built-in set does not define is a theme of its own — drop
`~/.local/state/thingd/themes/ocean.css` in place and it becomes a choice: write
`theme: ocean` on a project's entry in `projects.yaml`, or pick it in the root
picker's Edit dialog, which lists whatever these directories hold.

## Writing one

Redefine every token the default palette sets (see `web/src/styles/tokens.css`),
for both color schemes:

```css
[data-theme="ocean"] {
  --bg: …; --panel: …; --panel-2: …; --line: …;
  --ink: …; --ink-2: …; --ink-3: …;
  --amber: …;  /* the accent — the token is named for the default palette */
  --link: var(--amber);
  --todo: …; --doing: …; --done: …; --paused: …; --high: …;
}
@media (prefers-color-scheme: light) {
  [data-theme="ocean"] { /* the same tokens, tuned for light */ }
}
```

A partial file is fine too — anything left out falls through to the default
palette, which is what makes the override case above work. `--scrim`, the dim
behind a modal, is usually one to leave out: the default already carries a value
per color scheme.

Anchor the selector at `[data-theme="…"]` rather than `:root[data-theme="…"]`.
The board sets the attribute on the document element either way, but the picker
previews a theme by putting the attribute on a small element inside its edit
dialog, and a `:root`-anchored rule can only ever style the whole page. (A theme
written the `:root` way still colors a board — it just will not preview.)

Two things worth keeping true, since the board relies on them:

- `--ink`, `--ink-2`, and `--link` are body text, so aim for 4.5:1 against both
  `--bg` and `--panel`. The status colors are dots, rails, and chip backgrounds
  rather than text, so 3:1 is the bar there.
- Keep the status colors' meanings recognizable — `done` green, `high` red — and
  distinct enough from each other to read at a glance. The built-in themes tint
  them toward the theme's hue by only about 8% of the rotation for exactly this
  reason.

## How the built-in palettes were derived

Each is the default amber palette rotated in OKLCH at unchanged lightness: the
neutrals and accent rotate all the way to the theme's hue, the status colors
about 8% of it, and slate additionally mutes chroma. Lightness was nudged where a
rotated hue landed too close to its background, then each palette was checked
against the contrast bars above in both color schemes.

`--link` does not rotate. Its light-scheme blue is what reads as a hyperlink
whatever surrounds it; rotating it with the theme turned it rust, green, or
magenta.
