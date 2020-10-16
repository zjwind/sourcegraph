#!/usr/bin/env bash

cd "$(dirname "${BASH_SOURCE[0]}")/../.."
set -uex

ffmpeg -y -f x11grab -video_size 1280x1024 -i "$DISPLAY" -pix_fmt yuv420p e2e.mp4 >ffmpeg.log 2>&1 &

firefox

xclock

xeyes
