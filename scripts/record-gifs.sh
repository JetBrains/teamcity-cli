#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
TAPES_DIR="$ROOT_DIR/docs/tapes"
IMAGES_DIR="$ROOT_DIR/docs/images"

DARK_THEME="Dark+"
LIGHT_THEME="Catppuccin Latte"

usage() {
    echo "Usage: $0 [OPTIONS] [tape_name...]"
    echo ""
    echo "Generate GIF demos from VHS tape files in docs/tapes/"
    echo ""
    echo "Options:"
    echo "  --dark-only     Generate only dark theme variants"
    echo "  --light-only    Generate only light theme variants"
    echo "  --list          List available tape files"
    echo "  -h, --help      Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0                    # Generate all tapes, both themes"
    echo "  $0 run-list           # Generate only run-list tape"
    echo "  $0 --dark-only        # Generate all tapes, dark theme only"
    echo "  $0 --light-only auth-login run-list  # Specific tapes, light only"
}

gen_dark=true
gen_light=true
tapes=()

while [[ $# -gt 0 ]]; do
    case "$1" in
        --dark-only)  gen_light=false; shift ;;
        --light-only) gen_dark=false; shift ;;
        --list)
            for tape in "$TAPES_DIR"/*.tape; do
                basename "$tape" .tape
            done
            exit 0
            ;;
        -h|--help) usage; exit 0 ;;
        *) tapes+=("$1"); shift ;;
    esac
done

if ! command -v vhs &>/dev/null; then
    echo "Error: vhs is not installed. Install it with: brew install vhs"
    exit 1
fi

# Ensure 'teamcity' command is available (tapes use the full name)
if ! command -v teamcity &>/dev/null; then
    tc_bin="$(command -v tc 2>/dev/null || true)"
    if [[ -n "$tc_bin" ]]; then
        link_dir="$(dirname "$tc_bin")"
        echo "Creating symlink: $link_dir/teamcity -> $tc_bin"
        ln -sf "$tc_bin" "$link_dir/teamcity"
    else
        echo "Error: neither 'teamcity' nor 'tc' found in PATH"
        exit 1
    fi
fi

mkdir -p "$IMAGES_DIR"

record_tape() {
    local tape_file="$1"
    local theme="$2"
    local suffix="$3"
    local name
    name="$(basename "$tape_file" .tape)"
    # Writerside convention: light=name.gif, dark=name_dark.gif
    local output
    if [[ "$suffix" == "dark" ]]; then
        output="$IMAGES_DIR/${name}_dark.gif"
    else
        output="$IMAGES_DIR/${name}.gif"
    fi

    echo "Recording: $name ($suffix)..."
    sed -e "s|{{THEME}}|$theme|g" -e "s|{{OUTPUT}}|\"$output\"|g" "$tape_file" | vhs -

    if [[ -f "$output" ]]; then
        local size
        size=$(du -h "$output" | cut -f1)
        echo "  -> $output ($size)"
    else
        echo "  !! Failed to generate $output"
        return 1
    fi
}

# Determine which tapes to process
tape_files=()
if [[ ${#tapes[@]} -eq 0 ]]; then
    for tape in "$TAPES_DIR"/*.tape; do
        [[ -f "$tape" ]] && tape_files+=("$tape")
    done
else
    for name in "${tapes[@]}"; do
        tape="$TAPES_DIR/${name}.tape"
        if [[ -f "$tape" ]]; then
            tape_files+=("$tape")
        else
            echo "Warning: tape not found: $tape"
        fi
    done
fi

if [[ ${#tape_files[@]} -eq 0 ]]; then
    echo "No tape files found in $TAPES_DIR/"
    exit 1
fi

echo "Generating GIFs from ${#tape_files[@]} tape(s)..."
echo ""

failed=0
for tape in "${tape_files[@]}"; do
    if $gen_dark; then
        record_tape "$tape" "$DARK_THEME" "dark" || ((failed++))
    fi
    if $gen_light; then
        record_tape "$tape" "$LIGHT_THEME" "light" || ((failed++))
    fi
    echo ""
done

total_gifs=$(find "$IMAGES_DIR" -name "*.gif" -newer "$0" 2>/dev/null | wc -l | tr -d ' ')
echo "Done. Generated GIFs in $IMAGES_DIR/"

if [[ $failed -gt 0 ]]; then
    echo "Warning: $failed recording(s) failed."
    exit 1
fi
