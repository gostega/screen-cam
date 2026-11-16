# Changelog

All notable changes, newest first. One section per promoted release; work
lands in Unreleased first. Every release says what upgrading does to your data.

## Unreleased

- **Added:** capture engine — a viewport that follows the mouse is written to
  `/dev/video10` at a fixed 1280×720, so resizing never drops the browser's
  camera (`screen-cam follow`); `screen-cam pattern` writes colour bars for
  checks that don't touch the screen.

Upgrade: nothing to do
