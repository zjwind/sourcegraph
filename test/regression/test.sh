#!/usr/bin/env bash

cd "$(dirname "${BASH_SOURCE[0]}")/../.."
set -exo pipefail

# shellcheck disable=SC1091
source /root/.profile
Xvfb "$DISPLAY" -screen 0 1280x1024x24 &
x11vnc -display "$DISPLAY" -forever -rfbport 5900 >/x11vnc.log 2>&1 &

asdf install
yarn
yarn generate


test/regression/display.sh
