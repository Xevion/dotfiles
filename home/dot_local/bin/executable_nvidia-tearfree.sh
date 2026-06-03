#!/usr/bin/env bash
# Eliminate screen tearing on the proprietary NVIDIA driver under X11 by enabling
# ForceFullCompositionPipeline on every currently-connected output.
#
# Port-agnostic: the metamode is derived from whatever X has live (CurrentMetaMode)
# rather than hardcoding DPY indices or ports, so replugging monitors, changing
# resolutions, or adding/removing a display won't break it. No-ops cleanly when
# nvidia-settings is absent or no X metamode is available (e.g. Wayland/headless).
set -euo pipefail

command -v nvidia-settings >/dev/null 2>&1 || exit 0

# e.g. "id=50, switchable=no, source=nv-control :: DPY-5: nvidia-auto-select @1920x1080 +0+0 {ViewPortIn=...}, DPY-1: ... {...}"
meta=$(nvidia-settings -t -q CurrentMetaMode 2>/dev/null) || exit 0
meta=${meta%%$'\n'*}

# Bail unless the "<header> :: <metamode>" separator is present, then keep the payload.
case "$meta" in
  *" :: "*) meta=${meta#*" :: "} ;;
  *) exit 0 ;;
esac
[ -n "$meta" ] || exit 0

# Idempotent: strip any existing (Full)CompositionPipeline tokens, then inject
# ForceFullCompositionPipeline=On into every brace group.
clean=$(printf '%s' "$meta" | sed -E 's/(, *)?Force(Full)?CompositionPipeline=(On|Off)//g')
final=$(printf '%s' "$clean" | sed -E 's/\}/, ForceFullCompositionPipeline=On}/g')

exec nvidia-settings --assign CurrentMetaMode="$final"
