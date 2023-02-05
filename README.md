1. Build http server `./build_http.sh` and start `./httpserver`
2. Start transcoding `./ffmpeg.sh --source <url/file> --target http://localhost:8001/master.mpd [--encoder <libx264/h264_videotoolbox>] [--idr <int>]`
3. Play dash stream [http://localhost:8001/master.mpd](https://reference.dashif.org/dash.js/nightly/samples/dash-if-reference-player/index.html?mpd=http%3A%2F%2Flocalhost%3A8001%2Fmaster.mpd)
4. Play hls stream [http://localhost:8001/master.m3u8](https://hls-js.netlify.app/demo/?src=http%3A%2F%2Flocalhost%3A8001%2Fmaster.m3u8&demoConfig=eyJlbmFibGVTdHJlYW1pbmciOnRydWUsImF1dG9SZWNvdmVyRXJyb3IiOnRydWUsInN0b3BPblN0YWxsIjpmYWxzZSwiZHVtcGZNUDQiOmZhbHNlLCJsZXZlbENhcHBpbmciOi0xLCJsaW1pdE1ldHJpY3MiOi0xfQ==)
5. Run `./analyze.sh http://localhost:8001/master.mpd` to analyze GOP/I-frames structure