# GUI module layout

The GUI is split into these boundaries:

- `../../../wails.json`, `../../../main.go`: Wails build entry and asset embed shim. This
  remains at the repository root because Go embed patterns cannot reference
  parent directories from `cmd/nekocode-gui`, and Wails needs `interaction/gui/web/dist`
  embedded for production builds.
- `../app`: Wails bridge implementation. It owns session APIs,
  image loading, confirmation bridge, and event dispatch. The root `main.App`
  type is a small facade that keeps generated bindings stable at
  `wailsjs/go/main/App`.
- `../../../cmd/nekocode-tui`: TUI command entrypoint. It calls `tui.Run()`.
- `src/`: React application code.
- `src/components/session`: session list and switching UI.
- `src/components/run`: assistant run, tool activity, diffs, images, tasks, and
  reasoning UI.
- `src/hooks`: stateful Wails/event/session hooks.
- `src/lib`: thin Wails safety wrappers and shared utilities.
- `src/types`: front-end event/session contracts.
- `src/styles`: Tailwind entrypoint and theme tokens.
- `wailsjs/`: generated Wails bindings. Do not edit by hand.

The front-end package is intentionally self-contained in this directory. Run
`npm install`, `npm run build`, and `npm test` from `interaction/gui/web/`, not from the
repository root.
