#!/bin/sh
# Entry point of server/test.Dockerfile. As root it formats and mounts the
# filesystem images the volume-semantics tests need (vfat, exfat, ext4, and
# casefold ext4, as the server-volumes-linux CI job does), then runs each
# gate as the unprivileged runner and reports every step before exiting.
# LUMILIO_GATE_STEPS picks the steps (default: ci concurrency volumes).
set -u
steps="${LUMILIO_GATE_STEPS:-ci concurrency volumes}"
scratch="${LUMILIO_GATE_SCRATCH:-/scratch}"
mkdir -p "$scratch"
chown runner:runner "$scratch"
volumes="$scratch/volumes"
fixtures=""

unmount() {
    for point in casefold ext4 exfat vfat; do
        mountpoint -q "$volumes/$point" 2>/dev/null && umount "$volumes/$point"
    done
}
trap unmount EXIT

case " $steps " in *" volumes "*)
    uid="$(id -u runner)" gid="$(id -g runner)"
    rm -rf "$volumes" && mkdir -p "$volumes"
    for kind in vfat exfat ext4 casefold; do
        truncate -s 2G "$volumes/$kind.img" && mkdir -p "$volumes/$kind"
    done
    # utf8 is explicit: Ubuntu kernels (the CI runners) enable it by default
    # for vfat, Fedora's mount it iocharset=ascii and refuse non-ASCII names.
    mkfs.vfat -F 32 "$volumes/vfat.img" >/dev/null &&
        mount -o "loop,utf8,uid=$uid,gid=$gid" "$volumes/vfat.img" "$volumes/vfat" &&
        mkfs.exfat "$volumes/exfat.img" >/dev/null &&
        mount -o "loop,uid=$uid,gid=$gid" "$volumes/exfat.img" "$volumes/exfat" &&
        mkfs.ext4 -q "$volumes/ext4.img" &&
        mount -o loop "$volumes/ext4.img" "$volumes/ext4" &&
        chown runner:runner "$volumes/ext4" &&
        mkfs.ext4 -q -O casefold -E encoding=utf8 "$volumes/casefold.img" &&
        mount -o loop "$volumes/casefold.img" "$volumes/casefold" &&
        mkdir "$volumes/casefold/tree" && chattr +F "$volumes/casefold/tree" &&
        chown runner:runner "$volumes/casefold/tree" ||
        { echo "lumilio-linux-gate: cannot mount filesystem images; run the container with --privileged -v /dev:/dev" >&2; exit 2; }
    fixtures="vfat=$volumes/vfat;exfat=$volumes/exfat;ext4=$volumes/ext4;casefold=$volumes/casefold/tree"
    ;;
esac

status=0
for step in $steps; do
    case "$step" in
        ci) target=server:test:ci ;;
        concurrency) target=server:test:concurrency:ci ;;
        volumes) target=server:test:volumes ;;
        *) echo "lumilio-linux-gate: unknown step $step" >&2; exit 2 ;;
    esac
    # Only the volumes step sees the images, as in CI: with the variable set,
    # server:test:ci would run the volume tests too and push server/app past
    # Go's package timeout.
    step_fixtures=""
    [ "$step" = volumes ] && step_fixtures="$fixtures"
    echo "lumilio-linux-gate: $step ($target) starting"
    if su runner -s /bin/sh -c "cd /src && TMPDIR='$scratch' LUMILIO_VOLUME_FIXTURES='$step_fixtures' PATH='$PATH' GOPATH='$GOPATH' GOCACHE='$GOCACHE' task $target"; then
        echo "lumilio-linux-gate: $step ok"
    else
        echo "lumilio-linux-gate: $step FAILED"
        status=1
    fi
done
exit $status
