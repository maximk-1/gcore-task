#!/usr/bin/env bash

# usage:
# ffmpeg.sh --source <url/file> --target http://localhost:8001/master.mpd [--encoder <libx264/h264_videotoolbox>] [--idr <int>]

encoder=libx264
fps=25
idr=1
seg_duration=8

while getopts ":-:" optchar; do
  case "${optchar}" in
    -)
      case "${OPTARG}" in
          source) source="${!OPTIND}"; OPTIND=$(( $OPTIND + 1 ));;
          target) target="${!OPTIND}"; OPTIND=$(( $OPTIND + 1 ));;
          encoder) encoder="${!OPTIND}"; OPTIND=$(( $OPTIND + 1 ));;
          idr) idr="${!OPTIND}"; OPTIND=$(( $OPTIND + 1 ));;
          *) echo "Unknown option --${OPTARG}" >&2; exit;;
      esac;;
    *) echo "Unknown option -${OPTARG}" >&2; exit;;
  esac
done

hls_master_name="$(basename ${target%.*}.m3u8)"
gop=$((fps * idr))
seg_duration=$(( idr > seg_duration ? idr : seg_duration ))

filter_complex="[0:v]fps=fps=$fps,drawtext=text=%{localtime}:r=25:x=10:y=10:fontsize=36:fontcolor=white,split=3[p0][p1][p2];[p0]scale=854:-2[out0];[p1]scale=1280:-2[out1];[p2]scale=1920:-2[out2]"

case $encoder in
 libx264)
   h264opts_480="-x264opts:v:0 no-scenecut:ref=3 -profile:v:0 main -preset:v:0 medium -tune:v:0 zerolatency -coder:v:0 1"
   h264opts_720="-x264opts:v:1 no-scenecut:ref=3 -profile:v:1 main -preset:v:1 medium -tune:v:1 zerolatency -coder:v:1 1"
   h264opts_1080="-x264opts:v:2 no-scenecut:ref=3 -profile:v:2 high -preset:v:2 medium -tune:v:2 zerolatency -coder:v:2 1"
   ;;
 h264_videotoolbox)
   h264opts_480="-profile:v:0 main -coder:v:0 2 -realtime true -force_key_frames:v:0 expr:gte(t,n_forced*$idr)"
   h264opts_720="-profile:v:1 main -coder:v:1 2 -realtime true -force_key_frames:v:1 expr:gte(t,n_forced*$idr)"
   h264opts_1080="-profile:v:2 high -coder:v:2 2 -realtime true -force_key_frames:v:2 expr:gte(t,n_forced*$idr)"
   ;;
esac

bitrate_480="800k"
bitrate_720="1800k"
bitrate_1080="3600k"
profile_480="-c:v:0 $encoder -b:v:0 $bitrate_480 -maxrate:v:0 $bitrate_480 -bufsize:v:0 $bitrate_480 -flags:v:0 +cgop -r:v:0 $fps -g:v:0 $gop -keyint_min:v:0 $gop -bf:v:0 2 -pix_fmt:v:0 yuv420p $h264opts_480"
profile_720="-c:v:1 $encoder -b:v:1 $bitrate_720 -maxrate:v:1 $bitrate_720 -bufsize:v:1 $bitrate_720 -flags:v:1 +cgop -r:v:1 $fps -g:v:1 $gop -keyint_min:v:1 $gop -bf:v:1 2 -pix_fmt:v:1 yuv420p $h264opts_720"
profile_1080="-c:v:2 $encoder -b:v:2 $bitrate_1080 -maxrate:v:2 $bitrate_1080 -bufsize:v:2 $bitrate_1080 -flags:v:2 +cgop -r:v:2 $fps -g:v:2 $gop -keyint_min:v:2 $gop -bf:v:2 2 -pix_fmt:v:2 yuv420p $h264opts_1080"
audio="-c:a aac -b:a 96k -ar:a 44100 -ac:a 2"

ffmpeg -y -hwaccel auto -fflags +flush_packets+genpts -re -i "$source" \
  -filter_complex "$filter_complex" \
  -map [out0] -map [out1] -map [out2] -map 0:a \
  $profile_480 $profile_720 $profile_1080 $audio \
  -export_side_data prft \
  -dash_segment_type mp4 -streaming 1 -index_correction 1 -ldash 1 \
  -seg_duration $seg_duration -frag_duration 0.2 -window_size 5 \
  -use_timeline 0 -use_template 1 -write_prft 1 -target_latency 3 -utc_timing_url "/time" \
  -frag_type duration -adaptation_sets "id=0,streams=v id=1,streams=a" \
  -format_options "movflags=+frag_keyframe+empty_moov+cmaf" \
  -remove_at_exit 1 \
  -strict experimental -hls_playlist 1 -lhls 1 -hls_master_name "$hls_master_name" \
  -f dash -method POST -ignore_io_errors 1 -http_persistent 1 "$target"
