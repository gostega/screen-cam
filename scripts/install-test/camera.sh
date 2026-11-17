apt-get update -qq >/dev/null 2>&1; apt-get install -y -qq curl wget sudo procps python3-minimal >/dev/null 2>&1
useradd -m u; echo 'u ALL=(ALL) NOPASSWD:ALL' > /etc/sudoers.d/u
mkdir -p /srv && cp -r /t/rel /srv/ && (cd /srv && python3 -m http.server 8999 >/dev/null 2>&1 &); sleep 1
mv /usr/bin/curl /usr/bin/curl.off
su u -c "cd ~; SCREEN_CAM_RELEASES=http://127.0.0.1:8999/rel sh /t/install.sh --yes" 2>&1 | grep -v '^Get\|^Selecting\|^Preparing\|^Unpacking\|^Setting up\|^Processing\|^debconf' | tail -15; echo "[exit $?]"
echo "--- modprobe conf:"; cat /etc/modprobe.d/v4l2loopback-screen-cam.conf /etc/modules-load.d/v4l2loopback-screen-cam.conf
dpkg -l v4l2loopback-dkms | tail -1 | cut -c1-60; su u -c "~/.local/bin/screen-cam --version"
echo "--- dkms:"; dkms status; dpkg -l "linux-headers-*" | grep ^ii | cut -c1-70
