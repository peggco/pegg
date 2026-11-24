#!/usr/bin/env sh
# pegg installer — checks the system, downloads the latest GitHub release asset
# with a live progress bar, verifies it against checksums.txt, and installs the
# pegg binary.
#
# Usage:
#   curl -fsSL https://pegg.dev/install | sh
#   sh install.sh
#   VERSION=v1.2.3 PREFIX=$HOME/.local/bin sh install.sh
#
# Env vars:
#   VERSION           Release tag to install (default: latest)
#   PREFIX            Install directory (default: /usr/local/bin, falls back to ~/.local/bin)
#   NO_COLOR          Set to any value to disable colors
#   PEGG_PLUGINS      Comma-separated official plugins to install without prompting (none = skip)
#   PEGG_NO_PLUGINS   Set to any value to skip the plugin picker
#   PEGG_PLUGINS_URL  Plugin catalog URL (default: https://cdn.pegg.dev/plugins.json)
#   PEGG_NO_PATH      Set to any value to skip updating the shell profile PATH

set -eu

ESC=$(printf '\033')
TTY=0
[ -t 1 ] && TTY=1

if [ "$TTY" -eq 1 ] && [ -z "${NO_COLOR:-}" ]; then
	C_BOLD="${ESC}[1m"
	C_DIM="${ESC}[2m"
	C_GREEN="${ESC}[32m"
	C_RED="${ESC}[31m"
	C_YELLOW="${ESC}[33m"
	C_CYAN="${ESC}[36m"
	C_RESET="${ESC}[0m"
	CLEAR="${ESC}[2K"
else
	C_BOLD=""
	C_DIM=""
	C_GREEN=""
	C_RED=""
	C_YELLOW=""
	C_CYAN=""
	C_RESET=""
	CLEAR=""
fi

REPO="peggco/pegg"
BINARY="pegg"
DEFAULT_PREFIX="/usr/local/bin"
BAR_WIDTH=24
STEP_LABEL=""
PLUGINS_URL="${PEGG_PLUGINS_URL:-https://cdn.pegg.dev/plugins.json}"
DEFAULT_PLUGIN_KEYS="usage browser-use computer-use kanban"
PLUGIN_ROWS=8
PLUGIN_FRAME_LINES=$((PLUGIN_ROWS + 8))
PLUGIN_QUERY=""
PLUGIN_CURSOR=0
PLUGIN_SCROLL=0
PLUGIN_DRAWN=0
PLUGIN_VISIBLE_COUNT=0
PLUGIN_CANCELLED=0
PLUGIN_SAVED_STTY=""
PATH_MARK="# added by the pegg installer"

print_logo() {
	printf '%s' "$C_CYAN$C_BOLD"
	cat <<'EOF'

   @@@@@@@@@@@@@       @@@@@@@@@       @@@@@@@@@      @@@@@@@@@@
   @@@@@@@@@@@@@@    @@@@@@@@@@@@    @@@@@@@@@@@@@  @@@@@@@@@@@@@@
   @@@@@      @@@@@ @@@@      @@@@  @@@@@     @@@@@@@@@@      @@@@@
   @@@@        @@@@@@@@@@@@@@@@@@@@@@@@         @@@@@@         @@@@
   @@@@        @@@@@@@@@@@@@@@@@@@@@@@@         @@@@@@         @@@@
   @@@@@      @@@@  @@@@@      @    @@@@@     @@@@@@@@@@      @@@@@
   @@@@@@@@@@@@@@    @@@@@@@@@@@@@   @@@@@@@@@@@@@@  @@@@@@@@@@@@@@
   @@@@ @@@@@@@        @@@@@@@@@       @@@@@@@@@@@@    @@@@@@@@@@@@
   @@@@                                        @@@@            @@@@
   @@@@                              @@@@    @@@@@@  @@@@    @@@@@
   @@@@                              @@@@@@@@@@@@@  @@@@@@@@@@@@@
                                     @@@@@@@        @@@@@@@@
EOF
	printf '%s' "$C_RESET"
	printf '\n   %shands-free AI coding agent%s  %s·%s  %shttps://pegg.dev%s\n\n' \
		"$C_DIM" "$C_RESET" "$C_DIM" "$C_RESET" "$C_DIM" "$C_RESET"
}

die() {
	printf '\n   %s✗%s %s\n' "$C_RED" "$C_RESET" "$*" >&2
	exit 1
}

plugin_restore_tty() {
	if [ "$TTY" -eq 1 ]; then
		printf '%s[?25h' "$ESC"
	fi
	if [ -n "${PLUGIN_SAVED_STTY:-}" ]; then
		stty "$PLUGIN_SAVED_STTY" < /dev/tty 2>/dev/null || true
		PLUGIN_SAVED_STTY=""
	fi
}

cleanup() {
	plugin_restore_tty
	if [ -n "${tmpdir:-}" ]; then
		rm -rf "$tmpdir"
	fi
}

on_signal() {
	cleanup
	exit 130
}

begin_step() {
	STEP_LABEL=$1
	if [ "$TTY" -eq 1 ]; then
		printf '\r%s   %s%-24s%s' "$CLEAR" "$C_BOLD" "$STEP_LABEL" "$C_RESET"
	fi
}

draw_progress() {
	# $1 = percent (0-100), $2 = suffix text
	[ "$TTY" -eq 1 ] || return 0
	_pct=$1
	if [ "$_pct" -gt 100 ]; then _pct=100; fi
	if [ "$_pct" -lt 0 ]; then _pct=0; fi
	_fill=$(( _pct * BAR_WIDTH / 100 ))
	_empty=$(( BAR_WIDTH - _fill ))
	_fill_str=$(printf '%*s' "$_fill" '' | tr ' ' '=')
	_empty_str=$(printf '%*s' "$_empty" '' | tr ' ' '.')
	printf '\r%s   %s%-24s%s %s[%s%s%s]%s %3d%%  %s%s' \
		"$CLEAR" \
		"$C_BOLD" "$STEP_LABEL" "$C_RESET" \
		"$C_CYAN" "$_fill_str" "$C_DIM" "$_empty_str" "$C_RESET" \
		"$_pct" "$C_DIM" "$2$C_RESET"
}

draw_done() {
	# $1 = result text
	if [ "$TTY" -eq 1 ]; then
		printf '\r%s   %s%-24s%s %s✓%s %s\n' \
			"$CLEAR" "$C_BOLD" "$STEP_LABEL" "$C_RESET" "$C_GREEN" "$C_RESET" "$1"
	else
		printf '   %-24s %s\n' "$STEP_LABEL" "$1"
	fi
}

human() {
	_b=$1
	case $_b in ''|*[!0-9]*) _b=0 ;; esac
	if [ "$_b" -ge 1073741824 ]; then
		printf '%d.%d GB' $((_b / 1073741824)) $(( (_b % 1073741824) * 10 / 1073741824 ))
	elif [ "$_b" -ge 1048576 ]; then
		printf '%d.%d MB' $((_b / 1048576)) $(( (_b % 1048576) * 10 / 1048576 ))
	elif [ "$_b" -ge 1024 ]; then
		printf '%d KB' $((_b / 1024))
	else
		printf '%d B' "$_b"
	fi
}

bytes_of() {
	if [ ! -f "$1" ]; then
		printf '0'
		return 0
	fi
	_n=$(wc -c < "$1" 2>/dev/null | tr -d ' \t') || _n=0
	case ${_n:-} in ''|*[!0-9]*) _n=0 ;; esac
	printf '%s' "$_n"
}

run_animated() {
	if [ "$TTY" -eq 0 ]; then
		_rc=0
		"$@" || _rc=$?
		return "$_rc"
	fi
	_status="$tmpdir/.run.status"
	rm -f "$_status"
	( "$@"; printf '%s' "$?" > "$_status.tmp"; mv "$_status.tmp" "$_status" ) &
	_job=$!
	_frames=0
	_pct=0
	while [ ! -f "$_status" ]; do
		_frames=$((_frames + 1))
		if [ "$_pct" -lt 92 ]; then _pct=$((_pct + 9)); fi
		draw_progress "$_pct" ""
		sleep 0.05
	done
	_rc=$(cat "$_status" 2>/dev/null) || _rc=1
	case ${_rc:-} in ''|*[!0-9]*) _rc=1 ;; esac
	wait "$_job" 2>/dev/null || true
	_job=""
	return "$_rc"
}

fetch_url() {
	if command -v curl >/dev/null 2>&1; then
		curl -fsSL --connect-timeout 15 --retry 3 --retry-delay 2 "$1"
	elif command -v wget >/dev/null 2>&1; then
		wget -q --timeout=30 --tries=3 -O- "$1"
	else
		die "curl or wget is required"
	fi
}

download() {
	if command -v curl >/dev/null 2>&1; then
		curl -fsSL --connect-timeout 15 --retry 3 --retry-delay 2 \
			--speed-limit 1024 --speed-time 60 -o "$2" "$1"
	elif command -v wget >/dev/null 2>&1; then
		wget -q --timeout=30 --tries=3 -O "$2" "$1"
	else
		die "curl or wget is required"
	fi
}

fetch_latest_tag() {
	fetch_url "https://api.github.com/repos/$REPO/releases/latest" \
		| grep '"tag_name"' | head -n 1 | sed -E 's/.*"tag_name"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/'
}

fetch_asset() {
	# $1 = url, $2 = destination — live byte progress when a TTY is available.
	_total=0
	_hdrs=$(curl -fsSLI --connect-timeout 15 --retry 2 "$1" 2>/dev/null) || _hdrs=""
	if [ -n "$_hdrs" ] && command -v curl >/dev/null 2>&1; then
		_total=$(printf '%s' "$_hdrs" | tr -d '\r' \
			| awk 'tolower($1) == "content-length:" { v = $2 } END { print v }') || _total=0
	fi
	case ${_total:-} in ''|*[!0-9]*) _total=0 ;; esac

	if [ "$TTY" -eq 0 ]; then
		_rc=0
		download "$1" "$2" || _rc=$?
		return "$_rc"
	fi

	_status="$tmpdir/.fetch.status"
	rm -f "$_status"
	( download "$1" "$2"; printf '%s' "$?" > "$_status.tmp"; mv "$_status.tmp" "$_status" ) &
	_job=$!
	_frames=0
	while [ ! -f "$_status" ]; do
		_cur=$(bytes_of "$2")
		if [ "$_total" -gt 0 ]; then
			_pct=$(( _cur * 100 / _total ))
			if [ "$_pct" -gt 99 ]; then _pct=99; fi
			_suffix="$(human "$_cur") / $(human "$_total")"
		else
			_pct=$(( (_frames * 6) % 90 + 5 ))
			_suffix="$(human "$_cur")"
		fi
		draw_progress "$_pct" "$_suffix"
		_frames=$((_frames + 1))
		sleep 0.1
	done
	_rc=$(cat "$_status" 2>/dev/null) || _rc=1
	case ${_rc:-} in ''|*[!0-9]*) _rc=1 ;; esac
	wait "$_job" 2>/dev/null || true
	_job=""
	return "$_rc"
}

calc_sha() {
	# $1 = file, $2 = output file
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | awk '{print $1}' > "$2"
	elif command -v shasum >/dev/null 2>&1; then
		shasum -a 256 "$1" | awk '{print $1}' > "$2"
	elif command -v openssl >/dev/null 2>&1; then
		openssl dgst -sha256 "$1" | awk '{print $NF}' > "$2"
	else
		return 127
	fi
}

do_extract() {
	if [ "$ext" = "zip" ]; then
		command -v unzip >/dev/null 2>&1 || return 127
		unzip -q "$tmpdir/$asset.$ext" -d "$tmpdir/extract"
	else
		mkdir -p "$tmpdir/extract"
		tar -xzf "$tmpdir/$asset.$ext" -C "$tmpdir/extract"
	fi
}

do_install() {
	mkdir -p "$prefix" 2>/dev/null || true
	[ -d "$prefix" ] || return 1
	cp "$installed_bin" "$prefix/$installed_name"
}

lower() {
	printf '%s' "$1" | tr '[:upper:]' '[:lower:]'
}

plugin_catalog_load() {
	_catalog_raw="$tmpdir/plugins.json"
	if ! fetch_url "$PLUGINS_URL" > "$_catalog_raw" 2>/dev/null; then
		return 1
	fi
	: > "$tmpdir/plugins.all"

	if command -v jq >/dev/null 2>&1; then
		jq -r '.[] | select(.isOfficial == true)
			| [.id, .name, ((.description // "") | gsub("[\\t\\r\\n]+"; " ")),
			   ((.platforms // []) | join(",")), ((.arch // []) | join(",")),
			   ((.tags // []) | join(","))] | @tsv' \
			"$_catalog_raw" >> "$tmpdir/plugins.all" || return 1
	elif command -v python3 >/dev/null 2>&1; then
		if python3 - "$_catalog_raw" >> "$tmpdir/plugins.all" <<'PY'
import json
import sys

try:
    with open(sys.argv[1], encoding="utf-8") as fh:
        entries = json.load(fh)
except Exception:
    sys.exit(1)

for entry in entries:
    if not entry.get("isOfficial"):
        continue
    clean = lambda value: " ".join(str(value or "").split())
    fields = [
        entry.get("id", ""),
        clean(entry.get("name", "")),
        clean(entry.get("description", "")),
        ",".join(entry.get("platforms") or []),
        ",".join(entry.get("arch") or []),
        ",".join(entry.get("tags") or []),
    ]
    print("\t".join(fields))
PY
		then
			:
		else
			return 1
		fi
	else
		awk '
		function field(obj, key,   pat, p, i, c, out, esc) {
			pat = "\"" key "\"[ \t\r\n]*:[ \t\r\n]*\""
			if (!match(obj, pat)) return ""
			p = RSTART + RLENGTH
			out = ""; esc = 0
			for (i = p; i <= length(obj); i++) {
				c = substr(obj, i, 1)
				if (esc) {
					if (c == "n" || c == "t" || c == "r") out = out " "
					else out = out c
					esc = 0
				} else if (c == "\\") {
					esc = 1
				} else if (c == "\"") {
					break
				} else {
					out = out c
				}
			}
			return out
		}
		function arr(obj, key,   pat, p, i, c, out, esc) {
			pat = "\"" key "\"[ \t\r\n]*:[ \t\r\n]*\\["
			if (!match(obj, pat)) return ""
			p = RSTART + RLENGTH
			out = ""; esc = 0
			for (i = p; i <= length(obj); i++) {
				c = substr(obj, i, 1)
				if (esc) { out = out c; esc = 0 }
				else if (c == "\\") { esc = 1 }
				else if (c == "\"") { }
				else if (c == "]") { break }
				else if (c == ",") { out = out "," }
				else if (c == " " || c == "\t" || c == "\015" || c == "\n") { }
				else { out = out c }
			}
			return out
		}
		BEGIN {
			s = ""
			while ((getline line) > 0) s = s line "\n"
			depth = 0; instr = 0; esc = 0; start = 0; n = 0
			for (i = 1; i <= length(s); i++) {
				c = substr(s, i, 1)
				if (instr) {
					if (esc) esc = 0
					else if (c == "\\") esc = 1
					else if (c == "\"") instr = 0
					continue
				}
				if (c == "\"") { instr = 1; continue }
				if (c == "{") { depth++; if (depth == 1) start = i; continue }
				if (c == "}") {
					if (depth == 1 && start > 0) {
						objs[++n] = substr(s, start, i - start + 1)
						start = 0
					}
					depth--
				}
			}
			for (j = 1; j <= n; j++) {
				o = objs[j]
				if (!match(o, "\"isOfficial\"[ \t\r\n]*:[ \t\r\n]*true")) continue
				printf "%s\t%s\t%s\t%s\t%s\t%s\n", field(o, "id"), field(o, "name"), field(o, "description"), arr(o, "platforms"), arr(o, "arch"), arr(o, "tags")
			}
		}
		' "$_catalog_raw" >> "$tmpdir/plugins.all" || return 1
	fi

	: > "$tmpdir/plugins.tsv"
	while IFS="$(printf '\t')" read -r _id _name _desc _plats _ars _tags; do
		[ -n "$_id" ] || continue
		if [ -n "$_plats" ]; then
			case ",$_plats," in
				*",$os,"*) ;;
				*) continue ;;
			esac
		fi
		if [ -n "$_ars" ]; then
			case ",$_ars," in
				*",$arch,"*) ;;
				*) continue ;;
			esac
		fi
		printf '%s\t%s\t%s\t%s\n' "$_id" "$_name" "$_desc" "$_tags" >> "$tmpdir/plugins.tsv"
	done < "$tmpdir/plugins.all"

	if [ ! -s "$tmpdir/plugins.tsv" ] && [ -s "$tmpdir/plugins.all" ]; then
		cut -f1-4 "$tmpdir/plugins.all" > "$tmpdir/plugins.tsv"
		printf '   %splatform metadata incomplete; showing all official plugins%s\n' \
			"$C_DIM" "$C_RESET"
	fi

	: > "$tmpdir/plugins.sorted"
	for _key in $DEFAULT_PLUGIN_KEYS; do
		awk -F'\t' -v k="pegg-$_key" '
			substr($1, length($1) - length(k) + 1) == k { print; exit }
		' "$tmpdir/plugins.tsv" >> "$tmpdir/plugins.sorted"
	done
	awk -F'\t' -v keys=" $DEFAULT_PLUGIN_KEYS " '
		{
			n = split(keys, arr, " ")
			for (i = 1; i <= n; i++) {
				k = "pegg-" arr[i]
				if (substr($1, length($1) - length(k) + 1) == k) next
			}
			print
		}
	' "$tmpdir/plugins.tsv" >> "$tmpdir/plugins.sorted"
	mv "$tmpdir/plugins.sorted" "$tmpdir/plugins.tsv"
}

plugin_selected() {
	grep -Fxq "$1" "$tmpdir/selected.txt" 2>/dev/null
}

plugin_defaults() {
	for _key in $DEFAULT_PLUGIN_KEYS; do
		_id=$(awk -F'\t' -v k="pegg-$_key" \
			'substr($1, length($1) - length(k) + 1) == k { print $1; exit }' \
			"$tmpdir/plugins.tsv")
		if [ -n "$_id" ]; then
			printf '%s\n' "$_id" >> "$tmpdir/selected.txt"
		fi
	done
}

plugin_resolve() {
	# $1 = exact id or short key (usage, browser-use, ...)
	_tok=$1
	[ -n "$_tok" ] || return 0
	_id=$(awk -F'\t' -v k="$_tok" '$1 == k { print $1; exit }' "$tmpdir/plugins.tsv")
	if [ -z "$_id" ]; then
		_id=$(awk -F'\t' -v k="pegg-$_tok" \
			'substr($1, length($1) - length(k) + 1) == k { print $1; exit }' \
			"$tmpdir/plugins.tsv")
	fi
	printf '%s' "$_id"
}

plugin_visible_build() {
	: > "$tmpdir/visible.tsv"
	_needle=$(lower "$PLUGIN_QUERY")
	while IFS="$(printf '\t')" read -r _id _name _desc _tags; do
		_hay=$(lower "$_id $_name $_desc $_tags")
		case "$_hay" in
			*"$_needle"*) printf '%s\t%s\t%s\n' "$_id" "$_name" "$_desc" >> "$tmpdir/visible.tsv" ;;
		esac
	done < "$tmpdir/plugins.tsv"
	PLUGIN_VISIBLE_COUNT=$(wc -l < "$tmpdir/visible.tsv" | tr -d ' ')
}

plugin_move() {
	[ "$PLUGIN_VISIBLE_COUNT" -gt 0 ] || return 0
	PLUGIN_CURSOR=$((PLUGIN_CURSOR + $1))
	if [ "$PLUGIN_CURSOR" -lt 0 ]; then
		PLUGIN_CURSOR=$((PLUGIN_VISIBLE_COUNT - 1))
	fi
	if [ "$PLUGIN_CURSOR" -ge "$PLUGIN_VISIBLE_COUNT" ]; then
		PLUGIN_CURSOR=0
	fi
	if [ "$PLUGIN_CURSOR" -lt "$PLUGIN_SCROLL" ]; then
		PLUGIN_SCROLL=$PLUGIN_CURSOR
	fi
	if [ "$PLUGIN_CURSOR" -ge $((PLUGIN_SCROLL + PLUGIN_ROWS)) ]; then
		PLUGIN_SCROLL=$((PLUGIN_CURSOR - PLUGIN_ROWS + 1))
	fi
}

plugin_toggle() {
	[ "$PLUGIN_VISIBLE_COUNT" -gt 0 ] || return 0
	_id=$(sed -n "$((PLUGIN_CURSOR + 1))p" "$tmpdir/visible.tsv" | cut -f1)
	[ -n "$_id" ] || return 0
	if plugin_selected "$_id"; then
		grep -Fxv "$_id" "$tmpdir/selected.txt" > "$tmpdir/selected.tmp" || true
		mv "$tmpdir/selected.tmp" "$tmpdir/selected.txt"
	else
		printf '%s\n' "$_id" >> "$tmpdir/selected.txt"
	fi
}

_pl() {
	_line_no=$((_line_no + 1))
	printf '%s[2K%s' "$ESC" "$*"
	if [ "$_line_no" -lt "$PLUGIN_FRAME_LINES" ]; then
		printf '\n'
	fi
}

_plf() {
	_line_no=$((_line_no + 1))
	printf '%s[2K' "$ESC"
	_fmt=$1
	shift
	printf "$_fmt" "$@"
	if [ "$_line_no" -lt "$PLUGIN_FRAME_LINES" ]; then
		printf '\n'
	fi
}

plugin_render() {
	if [ "$PLUGIN_DRAWN" -gt 0 ]; then
		printf '%s[%dA\r' "$ESC" "$((PLUGIN_DRAWN - 1))"
	fi
	printf '%s[?2026h' "$ESC"
	_line_no=0

	_pl "   ${C_BOLD}Plugins${C_RESET}"
	_pl "   ${C_DIM}official only · ${PLUGINS_URL}${C_RESET}"
	_pl ""
	_pl "   Search: ${PLUGIN_QUERY}_"
	_pl ""
	_sel="
$(cat "$tmpdir/selected.txt" 2>/dev/null)
"
	if [ "$PLUGIN_VISIBLE_COUNT" -eq 0 ]; then
		_pl "   ${C_DIM}no official plugins match the search${C_RESET}"
		_idx=1
	else
		_idx=0
		while IFS="$(printf '\t')" read -r _id _name _desc; do
			if [ "$_idx" -lt "$PLUGIN_SCROLL" ]; then
				_idx=$((_idx + 1))
				continue
			fi
			if [ "$_idx" -ge $((PLUGIN_SCROLL + PLUGIN_ROWS)) ]; then
				break
			fi
			case "$_sel" in
				*"
$_id
"*) _mark="[x]" ;;
				*) _mark="[ ]" ;;
			esac
			_arrow=" "
			if [ "$_idx" -eq "$PLUGIN_CURSOR" ]; then
				_arrow=">"
			fi
			_plf '   %s %s %-20.20s %s%.42s%s' \
				"$_arrow" "$_mark" "$_name" "$C_DIM" "$_desc" "$C_RESET"
			_idx=$((_idx + 1))
		done < "$tmpdir/visible.tsv"
	fi
	while [ "$_idx" -lt "$PLUGIN_ROWS" ]; do
		_pl ""
		_idx=$((_idx + 1))
	done
	_pl ""
	_count=$(wc -l < "$tmpdir/selected.txt" | tr -d ' ')
	_pl "   ${C_DIM}${_count} selected${C_RESET}"
	_pl "   ${C_DIM}↑/↓ move · space toggle · enter install · type to search · esc skip${C_RESET}"
	printf '%s[?2026l' "$ESC"
	PLUGIN_DRAWN=$PLUGIN_FRAME_LINES
}

plugin_pick() {
	PLUGIN_QUERY=""
	PLUGIN_CURSOR=0
	PLUGIN_SCROLL=0
	PLUGIN_DRAWN=0
	PLUGIN_CANCELLED=0
	plugin_defaults
	plugin_visible_build

	PLUGIN_SAVED_STTY=$(stty -g < /dev/tty) || return 1
	stty -icanon -echo -isig min 1 time 0 < /dev/tty || return 1
	printf '%s[?25l' "$ESC"

	while :; do
		plugin_render
		_byte=$(dd bs=1 count=1 < /dev/tty 2>/dev/null | tr -d '\000')
		case "$_byte" in
			"")
				break
				;;
			"$ESC")
				stty min 0 time 1 < /dev/tty 2>/dev/null || true
				_b2=$(dd bs=1 count=1 < /dev/tty 2>/dev/null) || _b2=""
				stty min 1 time 0 < /dev/tty 2>/dev/null || true
				if [ "$_b2" != "[" ]; then
					PLUGIN_CANCELLED=1
					break
				fi
				_b3=$(dd bs=1 count=1 < /dev/tty 2>/dev/null) || break
				case "$_b3" in
					A) plugin_move -1 ;;
					B) plugin_move 1 ;;
				esac
				;;
			" ")
				plugin_toggle
				;;
			"$(printf '\r')"|"$(printf '\n')")
				break
				;;
			"$(printf '\003')")
				PLUGIN_CANCELLED=1
				break
				;;
			"$(printf '\177')"|"$(printf '\010')")
				if [ -n "$PLUGIN_QUERY" ]; then
					PLUGIN_QUERY=${PLUGIN_QUERY%?}
					PLUGIN_CURSOR=0
					PLUGIN_SCROLL=0
					plugin_visible_build
				fi
				;;
			*)
				case "$_byte" in
					[a-zA-Z0-9._/-])
						PLUGIN_QUERY="$PLUGIN_QUERY$_byte"
						PLUGIN_CURSOR=0
						PLUGIN_SCROLL=0
						plugin_visible_build
						;;
				esac
				;;
		esac
	done

	plugin_restore_tty
	if [ "$PLUGIN_DRAWN" -gt 0 ]; then
		printf '\n'
	fi
	[ "$PLUGIN_CANCELLED" -eq 0 ]
}

plugin_install_selected() {
	_count=0
	_failed=0
	if [ -s "$tmpdir/selected.txt" ]; then
		while IFS= read -r _id; do
			[ -n "$_id" ] || continue
			begin_step "Plugin"
			if _out=$("$prefix/$installed_name" plugins install "$_id" 2>&1); then
				draw_done "$_id"
				_count=$((_count + 1))
			else
				_failed=$((_failed + 1))
				if [ "$TTY" -eq 1 ]; then
					printf '\r%s   %s✗%s %s\n' "$CLEAR" "$C_RED" "$C_RESET" "$_id"
				else
					printf '   ✗ %s\n' "$_id"
				fi
				_last=$(printf '%s\n' "$_out" | grep -v '^[[:space:]]*$' | tail -n 1)
				if [ -n "$_last" ]; then
					printf '     %s%s%s\n' "$C_DIM" "$_last" "$C_RESET"
				fi
			fi
		done < "$tmpdir/selected.txt"
	fi
	if [ "$_count" -gt 0 ]; then
		printf '   %s%d plugin(s) installed%s\n' "$C_DIM" "$_count" "$C_RESET"
	elif [ "$_failed" -eq 0 ]; then
		printf '   %sno plugins selected%s\n' "$C_DIM" "$C_RESET"
	fi
}

install_plugins() {
	[ -n "${PEGG_NO_PLUGINS:-}" ] && return 0

	if ! plugin_catalog_load; then
		printf '   %s!%s could not load the plugin catalog; skipping plugins\n' \
			"$C_YELLOW" "$C_RESET"
		return 0
	fi
	if [ ! -s "$tmpdir/plugins.tsv" ]; then
		printf '   %sno official plugins available for %s/%s%s\n' \
			"$C_DIM" "$os" "$arch" "$C_RESET"
		return 0
	fi

	: > "$tmpdir/selected.txt"
	if [ -n "${PEGG_PLUGINS:-}" ]; then
		_old_ifs=$IFS
		IFS=","
		for _tok in $PEGG_PLUGINS; do
			_tok=$(printf '%s' "$_tok" | tr -d ' ')
			[ -n "$_tok" ] || continue
			if [ "$_tok" = "none" ]; then
				continue
			fi
			_id=$(plugin_resolve "$_tok")
			if [ -n "$_id" ]; then
				printf '%s\n' "$_id" >> "$tmpdir/selected.txt"
			else
				printf '   %s!%s unknown plugin: %s\n' "$C_YELLOW" "$C_RESET" "$_tok"
			fi
		done
		IFS=$_old_ifs
	elif [ "$TTY" -eq 0 ] || [ ! -r /dev/tty ]; then
		printf '   %splugins skipped (non-interactive)%s\n' "$C_DIM" "$C_RESET"
		return 0
	fi

	if [ -z "${PEGG_PLUGINS:-}" ]; then
		if ! plugin_pick; then
			printf '   %splugin selection skipped%s\n' "$C_DIM" "$C_RESET"
			return 0
		fi
	fi

	plugin_install_selected
}

shell_name() {
	_sh=${SHELL:-}
	if [ -z "$_sh" ]; then
		printf 'sh'
		return 0
	fi
	basename "$_sh"
}

# Prints the profile file that should carry the PATH export for the current
# login shell, or nothing when there is no sensible candidate.
path_profile() {
	case $(shell_name) in
		zsh)
			printf '%s' "${ZDOTDIR:-$HOME}/.zshrc"
			;;
		bash)
			if [ "$os" = "darwin" ]; then
				# macOS terminals start login shells, which read .bash_profile
				# (then .bash_login, then .profile) but never .bashrc.
				if [ -f "$HOME/.bash_profile" ]; then
					printf '%s' "$HOME/.bash_profile"
				elif [ -f "$HOME/.bash_login" ]; then
					printf '%s' "$HOME/.bash_login"
				elif [ -f "$HOME/.profile" ]; then
					printf '%s' "$HOME/.profile"
				else
					printf '%s' "$HOME/.bash_profile"
				fi
			else
				printf '%s' "$HOME/.bashrc"
			fi
			;;
		fish)
			printf '%s' "${XDG_CONFIG_HOME:-$HOME/.config}/fish/conf.d/pegg.fish"
			;;
		ksh|mksh|dash|sh|ash)
			printf '%s' "$HOME/.profile"
			;;
		*)
			printf ''
			;;
	esac
}

path_hint() {
	if [ "$(shell_name)" = "fish" ]; then
		printf '   %srestart your shell, or run: fish_add_path %s%s\n' \
			"$C_DIM" "$prefix" "$C_RESET"
	else
		printf '   %srestart your shell, or run: export PATH="%s:$PATH"%s\n' \
			"$C_DIM" "$prefix" "$C_RESET"
	fi
}

path_manual() {
	printf '   %s→ add %s to your PATH%s\n' "$C_YELLOW" "$prefix" "$C_RESET"
	path_hint
}

path_write() {
	# $1 = profile file. Appends an idempotent PATH export guarded by PATH_MARK.
	_profile=$1
	_dir=$(dirname "$_profile")
	mkdir -p "$_dir" 2>/dev/null || true
	[ -d "$_dir" ] || return 1
	if [ -f "$_profile" ] && grep -Fq "$PATH_MARK" "$_profile" 2>/dev/null; then
		return 0
	fi
	{
		printf '\n%s\n' "$PATH_MARK"
		if [ "$(shell_name)" = "fish" ]; then
			printf 'fish_add_path %s\n' "$prefix"
		else
			printf 'case ":$PATH:" in\n'
			printf '\t*":%s:"*) ;;\n' "$prefix"
			printf '\t*) export PATH="%s:$PATH" ;;\n' "$prefix"
			printf 'esac\n'
		fi
	} >> "$_profile" 2>/dev/null || return 1
}

# The install prefix is often ~/.local/bin, which is not on the default PATH on
# macOS (and on many Linux distros), so the binary would be unreachable right
# after a "successful" install. Wire it into the login shell's profile.
ensure_on_path() {
	case ":$PATH:" in
		*":$prefix:"*) return 0 ;;
	esac

	if [ -n "${PEGG_NO_PATH:-}" ]; then
		path_manual
		return 0
	fi

	_target=$(path_profile)
	if [ -z "$_target" ]; then
		path_manual
		return 0
	fi

	if path_write "$_target"; then
		printf '   %s%s added to PATH in %s%s\n' "$C_DIM" "$prefix" "$_target" "$C_RESET"
		path_hint
	else
		path_manual
	fi
}

print_logo

begin_step "System"
os_raw=$(uname -s)
arch_raw=$(uname -m)

case "$os_raw" in
	Linux)  os="linux" ;;
	Darwin) os="darwin" ;;
	MINGW*|MSYS*|CYGWIN*) os="windows" ;;
	*) die "unsupported operating system: $os_raw (supported: linux, darwin, windows)" ;;
esac

case "$arch_raw" in
	x86_64|amd64)   arch="amd64" ;;
	aarch64|arm64)  arch="arm64" ;;
	*) die "unsupported architecture: $arch_raw (supported: amd64, arm64)" ;;
esac

asset="${BINARY}_${os}_${arch}"
ext="tar.gz"
[ "$os" = "windows" ] && ext="zip"
draw_done "$os/$arch · $asset.$ext"

begin_step "Release"
if [ -n "${VERSION:-}" ]; then
	tag="$VERSION"
else
	tag=$(fetch_latest_tag) || die "failed to fetch the latest release tag"
fi
[ -n "$tag" ] || die "failed to fetch the latest release tag"
draw_done "$tag"

begin_step "Download"
tmpdir=$(mktemp -d)
trap 'cleanup' EXIT
trap 'on_signal' INT TERM HUP

base_url="https://github.com/$REPO/releases/download/$tag"
asset_url="$base_url/$asset.$ext"

fetch_asset "$asset_url" "$tmpdir/$asset.$ext" \
	|| die "failed to download $asset.$ext (release $tag may not exist)"
draw_done "$(human "$(bytes_of "$tmpdir/$asset.$ext")")"

begin_step "Verify"
if ! download "$base_url/checksums.txt" "$tmpdir/checksums.txt" 2>/dev/null; then
	download "https://github.com/$REPO/releases/latest/download/checksums.txt" "$tmpdir/checksums.txt" \
		|| die "failed to download checksums.txt"
fi

expected=$(grep -E "(^|[/ ])$asset\.$ext\$" "$tmpdir/checksums.txt" | awk '{print $1}' | head -n 1)
[ -n "$expected" ] || die "no checksum entry found for $asset.$ext"

run_animated calc_sha "$tmpdir/$asset.$ext" "$tmpdir/actual.sha" \
	|| die "no sha256 tool found (need sha256sum, shasum, or openssl)"
actual=$(cat "$tmpdir/actual.sha" 2>/dev/null) || actual=""
[ "$actual" = "$expected" ] || die "checksum mismatch for $asset.$ext"
draw_done "sha256 matched"

begin_step "Extract"
run_animated do_extract || die "failed to extract $asset.$ext (is unzip installed?)"

installed_bin="$tmpdir/extract/$BINARY"
if [ "$os" = "windows" ] && [ ! -f "$installed_bin" ]; then
	installed_bin="$tmpdir/extract/$BINARY.exe"
fi
[ -f "$installed_bin" ] || installed_bin=$(find "$tmpdir/extract" -type f \( -name "$BINARY" -o -name "$BINARY.exe" \) | head -n 1)
[ -n "$installed_bin" ] && [ -f "$installed_bin" ] || die "$BINARY binary not found inside $asset.$ext"
chmod +x "$installed_bin"
draw_done "$BINARY unpacked"

begin_step "Install"
prefix="${PREFIX:-}"
if [ -z "$prefix" ]; then
	if [ -w "$DEFAULT_PREFIX" ]; then
		prefix="$DEFAULT_PREFIX"
	else
		prefix="$HOME/.local/bin"
	fi
fi

installed_name="$BINARY"
[ "$os" = "windows" ] && installed_name="$BINARY.exe"

mkdir -p "$prefix" 2>/dev/null || true
if [ ! -d "$prefix" ] || [ ! -w "$prefix" ]; then
	die "cannot write to $prefix (try: PREFIX=\$HOME/.local/bin sh install.sh, or run with sudo)"
fi

run_animated do_install \
	|| die "failed to install to $prefix (close any running pegg process and retry, or set PREFIX=\$HOME/.local/bin)"
chmod +x "$prefix/$installed_name"
draw_done "$prefix/$installed_name"

install_plugins

printf '\n'
printf '   %s✓ pegg %s installed successfully%s\n' "$C_GREEN$C_BOLD" "$tag" "$C_RESET"
printf '   %s%s%s\n' "$C_DIM" "$prefix/$installed_name" "$C_RESET"
_version=$("$prefix/$installed_name" version 2>/dev/null) || _version=""
if [ -n "$_version" ]; then
	printf '   %s%s%s\n' "$C_DIM" "$_version" "$C_RESET"
fi

ensure_on_path

printf '\n   %snext:%s pegg login    %s·%s    pegg\n\n' \
	"$C_BOLD" "$C_RESET" "$C_DIM" "$C_RESET"
