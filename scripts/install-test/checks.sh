set -u
apt-get update -qq >/dev/null 2>&1; apt-get install -y -qq curl procps >/dev/null 2>&1
useradd -m u; cd /home/u
r() { su u -c "cd ~; SCREEN_CAM_RELEASES=$1 sh /t/install.sh $2" 2>&1 < /dev/null; echo "[exit $?]"; }
P=0; F=0; ok(){ if eval "$2"; then echo "PASS $1"; P=$((P+1)); else echo "FAIL $1"; F=$((F+1)); fi; }
echo "== fresh"; out=$(r file:///t/rel ""); echo "$out"
ok "fresh exits 0" 'echo "$out" | grep -q "\[exit 0\]"'
ok "binary runs" '[ "$(su u -c "~/.local/bin/screen-cam --version")" = "screen-cam v0.2.0-test" ]'
ok "menu entry" 'grep -q "Exec=/home/u/.local/bin/screen-cam" /home/u/.local/share/applications/screen-cam.desktop'
ok "autostart" '[ -f /home/u/.config/autostart/screen-cam.desktop ]'
ok "camera hint without sudo" 'echo "$out" | grep -q "modprobe v4l2loopback"'
ok "PATH note" 'echo "$out" | grep -q "not on your PATH"'
echo "== rerun"; out=$(r file:///t/rel ""); ok "rerun exits 0" 'echo "$out" | grep -q "\[exit 0\]"'
echo "== pinned"; out=$(r file:///t/rel "--version v0.2.0-test --no-camera-module"); ok "pinned exits 0" 'echo "$out" | grep -q "\[exit 0\]"'
ok "no-camera-module skips" '! echo "$out" | grep -q "isn.t set up"'
echo "== missing tag"; out=$(r file:///t/rel "--version v9.9.9"); echo "$out" | tail -2; ok "missing tag fails clearly" 'echo "$out" | grep -q "no published release for v9.9.9"'
echo "== uninstall"; out=$(r file:///t/rel "--uninstall"); ok "uninstall removes" '[ ! -e /home/u/.local/bin/screen-cam ] && [ ! -e /home/u/.config/autostart/screen-cam.desktop ]'
echo "== tampered"; out=$(r file:///t/bad ""); echo "$out" | tail -2; ok "tampered refused" 'echo "$out" | grep -q "checksum mismatch"'
ok "tampered installs nothing" '[ ! -e /home/u/.local/bin/screen-cam ]'
echo "== wget only"; apt-get install -y -qq wget >/dev/null 2>&1; mv /usr/bin/curl /usr/bin/curl.off
mkdir -p /srv && cp -r /t/rel /srv/ && (cd /srv && python3 -m http.server 8999 >/dev/null 2>&1 &) ; sleep 1
if command -v python3 >/dev/null; then out=$(r http://127.0.0.1:8999/rel ""); ok "wget path" 'echo "$out" | grep -q "\[exit 0\]"'; else echo "SKIP wget (no python3 for a server)"; fi
echo "RESULT $P passed, $F failed"
