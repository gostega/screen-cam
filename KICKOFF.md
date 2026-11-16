# Kickoff — screen-cam M2: the tray app

Paste the block below into a fresh session started in
`~/git/james-github/screen-cam`. Before pasting it, check that
`docs/design.md` §Design review records your verdict. If it still says
"pending", review the page first.

---

You are building M2 of screen-cam, a Go tray app that makes a mouse-following
screen viewport into a webcam (/dev/video10) for testing ubervent's QR scan.

Read, in order: AGENTS.md, spec.md, docs/adr/0001-pure-go-fixed-output.md,
docs/design.md (the **Design review** table is the authority on what was
accepted; the review page https://claude.ai/artifact/HAUvCxn5zaX6zKdeJStgBo
holds the figures and callouts it refers to). Then
`backlog task list --plain` (fnm PATH in AGENTS.md).

Goal: TASK-4 to TASK-9 in dependency order, built to the accepted callouts
of figures tray-menu, screen-live and save-preset. Keep each callout's
behaviour as written; for example, the preset name prompt is a zenity entry
pre-filled with the size, not a settings window. TASK-2 (MIT-SHM capture)
may go first if the frame rate holds up the outline work.

Excluded: Wayland, recording, audio, changing the 1280x720 output live, any
change to the kernel-module installer beyond D8's summary line.

Evidence: package tests for presets, viewport (steps, aspect cycle) and
shortcut parsing; device read-back checks using `screen-cam pattern` or a
window you draw yourself. Never capture James's live desktop. Run the real
tray on this machine (Cinnamon, X11, 2560x1440): the icon appears, every menu
item works, every shortcut works with another app focused. TASK-10 is James's
own acceptance. Leave it pending and hand him the steps.

Authority: build on branch m2-tray, commit locally per task with the
changelog trailers, never push. Ask before creating the GitHub remote. End
with a session report (numbered callouts on screenshots of the real tray
menu and outline, captured over a test window, not his desktop), and tag
v0.1.0-rc1 on the branch.
