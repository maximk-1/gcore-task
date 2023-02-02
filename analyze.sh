#!/usr/bin/env bash

# usage:
# analyze.sh <dash_url>

ffprobe -v 0 \
  -select_streams v -skip_frame nokey \
  -show_entries frame=stream_index,pts,pts_time,pict_type \
  -of compact \
  -read_intervals "%+60" \
  "$1"
