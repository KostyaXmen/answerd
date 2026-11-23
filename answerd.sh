#!/bin/sh
# answerd installer / uninstaller.
set -eu

usage() {
    cat <<'EOF'
Usage:
  ./answerd.sh --install [--autostart|--no-autostart] [--no-start] [--restart-plasma]
  ./answerd.sh --uninstall [--purge] [--restart-plasma]

Actions:
  --install        build and install the daemon, config (if missing), widget
  --uninstall      stop the daemon and remove binary, widget and autostart

Autostart (install only, default: --autostart):
  --autostart      add the KDE autostart entry
  --no-autostart   do not add the KDE autostart entry

Other:
  --no-start        do not start the daemon after --install
  --restart-plasma  restart plasmashell after changing the widget
  --purge           with --uninstall: also remove config and cache
  -h, --help        show this help
EOF
}

ACTION=""
AUTOSTART=1
START=1
RESTART_PLASMA=0
PURGE=0
AUTOSTART_GIVEN=0

for arg in "$@"; do
    case "$arg" in
        --install) ACTION=install ;;
        --uninstall) ACTION=uninstall ;;
        --autostart) AUTOSTART=1; AUTOSTART_GIVEN=1 ;;
        --no-autostart) AUTOSTART=0; AUTOSTART_GIVEN=1 ;;
        --no-start) START=0 ;;
        --restart-plasma) RESTART_PLASMA=1 ;;
        --purge) PURGE=1 ;;
        -h|--help) usage; exit 0 ;;
        *)
            echo "unknown option: $arg" >&2
            usage
            exit 2
            ;;
    esac
done

if [ -z "$ACTION" ]; then
    echo "error: specify --install or --uninstall" >&2
    usage
    exit 2
fi

case "$ACTION" in
    install)
        if [ "$PURGE" -eq 1 ]; then
            echo "error: --purge is only valid with --uninstall" >&2
            exit 2
        fi
        ;;
    uninstall)
        if [ "$AUTOSTART_GIVEN" -eq 1 ]; then
            echo "note: --autostart/--no-autostart only affect --install; autostart is always removed" >&2
        fi
        ;;
esac

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cd "$ROOT"

BIN_DIR="$HOME/.local/bin"
CONFIG_HOME="${XDG_CONFIG_HOME:-$HOME/.config}"
DATA_HOME="${XDG_DATA_HOME:-$HOME/.local/share}"
CACHE_DIR="$HOME/.cache/answerd"
CFG_DIR="$CONFIG_HOME/answerd"
PLASMOID_DIR="$DATA_HOME/plasma/plasmoids/answerd"
AUTOSTART_FILE="$CONFIG_HOME/autostart/answerd.desktop"
SYSTEMD_UNIT="$CONFIG_HOME/systemd/user/answerd.service"

need() {
    if ! command -v "$1" >/dev/null 2>&1; then
        echo "error: '$1' not found. $2" >&2
        exit 1
    fi
}

# PIDs of answerd processes whose executable is our installed binary or the
# repo build. Other processes with "answerd" in their command line are ignored.
daemon_pids() {
    for pid in $(pgrep -f '[a]nswerd run' 2>/dev/null || true); do
        exe=$(readlink "/proc/$pid/exe" 2>/dev/null || true)
        case "$exe" in
            "$BIN_DIR/answerd"|"$BIN_DIR/answerd (deleted)"|*"/daemon/answerd"*)
                echo "$pid"
                ;;
        esac
    done
}

stop_daemon() {
    pids=$(daemon_pids)
    [ -z "$pids" ] && return 0

    echo "==> stopping daemon ($pids)"
    kill $pids 2>/dev/null || true
    i=0
    while [ -n "$(daemon_pids)" ] && [ "$i" -lt 20 ]; do
        sleep 0.2
        i=$((i + 1))
    done
    remaining=$(daemon_pids)
    if [ -n "$remaining" ]; then
        kill -9 $remaining 2>/dev/null || true
    fi
}

start_daemon() {
    if [ -n "$(daemon_pids)" ]; then
        echo "==> daemon already running"
        return 0
    fi
    echo "==> starting daemon"
    nohup "$BIN_DIR/answerd" run >/dev/null 2>&1 &
}

install_daemon() {
    need go "Install Go >= 1.22 (pacman -S go / apt install golang-go)."
    need wl-paste "Install wl-clipboard (pacman -S wl-clipboard / apt install wl-clipboard)."
    need spectacle "Install spectacle (pacman -S spectacle / apt install spectacle)."

    mkdir -p "$BIN_DIR" "$CFG_DIR" "$CONFIG_HOME/autostart"

    echo "==> building daemon"
    (
        cd daemon
        go build -trimpath -o "$BIN_DIR/answerd" .
    )

    echo "==> config"
    if [ -f "$CFG_DIR/config.json" ]; then
        echo "    exists, left untouched: $CFG_DIR/config.json"
    else
        cp daemon/config.example.json "$CFG_DIR/config.json"
        chmod 600 "$CFG_DIR/config.json"
        # Never leave the "sk-..." placeholder behind: an empty key gives a
        # clear "API key is not set" error instead of a 401 from the API.
        sed -i 's|^  "api_key".*|  "api_key": "",|' "$CFG_DIR/config.json"

        if [ -n "${DEEPSEEK_API_KEY:-}" ]; then
            echo "    DEEPSEEK_API_KEY is set; api_key in file left empty"
        elif [ -t 0 ]; then
            printf "DeepSeek API key (Enter to skip): "
            stty -echo 2>/dev/null || true
            read -r api_key || api_key=""
            stty echo 2>/dev/null || true
            printf "\n"
            if [ -n "$api_key" ]; then
                escaped=$(printf '%s' "$api_key" | sed 's/[&|]/\\&/g')
                sed -i "s|^  \"api_key\".*|  \"api_key\": \"$escaped\",|" "$CFG_DIR/config.json"
            fi
        fi
    fi

    echo "==> autostart"
    if [ "$AUTOSTART" -eq 1 ]; then
        cp daemon/packaging/answerd.desktop "$AUTOSTART_FILE"
        echo "    enabled: $AUTOSTART_FILE"
    else
        rm -f "$AUTOSTART_FILE"
        echo "    disabled"
    fi

    echo "==> widget"
    mkdir -p "$DATA_HOME/plasma/plasmoids"
    rm -rf "$PLASMOID_DIR"
    cp -r widget "$PLASMOID_DIR"
    qml="$PLASMOID_DIR/contents/ui/DigitalClock.qml"
    sed_escape() { printf '%s' "$1" | sed 's/[&|]/\\&/g'; }
    sed -i "s|@ANSWERD_BIN@|$(sed_escape "$BIN_DIR/answerd")|g" "$qml"
    sed -i "s|@ANSWERD_LATEST@|$(sed_escape "$CACHE_DIR/latest.json")|g" "$qml"
    if grep -q '@ANSWERD_' "$qml"; then
        echo "error: unresolved placeholders in $qml" >&2
        exit 1
    fi

    stop_daemon
    if [ "$START" -eq 1 ]; then
        start_daemon
    fi

    cat <<EOF

Done. Next steps:
  1. If you skipped the key, put it into $CFG_DIR/config.json.
  2. Restart Plasma: systemctl --user restart plasma-plasmashell.service
  3. Add the "Answerd Clock" widget to the panel.
  4. In widget settings: Show date = on, Date format = Custom, MM/dd/yy.
EOF
}

uninstall_daemon() {
    stop_daemon

    if command -v systemctl >/dev/null 2>&1; then
        systemctl --user disable --now answerd 2>/dev/null || true
    fi

    echo "==> removing files"
    rm -f "$SYSTEMD_UNIT"
    rm -f "$BIN_DIR/answerd"
    rm -rf "$PLASMOID_DIR"
    rm -f "$AUTOSTART_FILE"

    if [ "$PURGE" -eq 1 ]; then
        rm -rf "$CFG_DIR" "$CACHE_DIR"
        echo "    removed config and cache"
    else
        echo "    kept config: $CFG_DIR"
        echo "    kept cache:  $CACHE_DIR (use --purge to remove)"
    fi
}

case "$ACTION" in
    install) install_daemon ;;
    uninstall) uninstall_daemon ;;
esac

if [ "$RESTART_PLASMA" -eq 1 ]; then
    if command -v systemctl >/dev/null 2>&1 && systemctl --user is-active --quiet plasma-plasmashell.service; then
        echo "==> restarting plasmashell"
        systemctl --user restart plasma-plasmashell.service
    elif command -v plasmashell >/dev/null 2>&1; then
        echo "==> restarting plasmashell"
        (plasmashell --replace >/dev/null 2>&1 &)
    fi
fi
