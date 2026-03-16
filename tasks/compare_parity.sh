#!/bin/bash

C_DIR="/home/darkliquid/Projects/ironwail/Quake"
GO_DIR="/home/darkliquid/Projects/ironwail-go"

check_file() {
    local c_file=$1
    local go_pkg=$2
    echo "### Analysis of $c_file (Mapped to $go_pkg)"
    echo ""
    
    # Extract function names using regex (rough approximation for C)
    funcs=$(grep -E '^[a-zA-Z_][a-zA-Z0-9_*\s]+ [a-zA-Z_0-9]+\s*\(' "$C_DIR/$c_file" | grep -v "{" | grep -v ";" | awk '{for(i=1;i<=NF;i++) if($i ~ /\(/) {gsub(/\(.*/, "", $i); print $i}}' | grep -vE '^(if|while|switch|for|return)$' | sort | uniq)
    
    local missing=0
    local found=0
    
    for f in $funcs; do
        # Strip common C prefixes for the Go search
        search_name=$(echo "$f" | sed -e 's/^SV_//i' -e 's/^CL_//i' -e 's/^Net_//i' -e 's/^R_//i' -e 's/^S_//i' -e 's/^Host_//i' -e 's/^Sys_//i')
        
        # Search the Go codebase for the function (case-insensitive)
        if grep -riE "func (\([^)]+\) )?_?$search_name\b" "$GO_DIR/internal/$go_pkg" "$GO_DIR/cmd" > /dev/null 2>&1; then
            found=$((found+1))
        else
            # Try a broader search across the whole Go codebase
            if grep -riE "func (\([^)]+\) )?_?$search_name\b" "$GO_DIR" > /dev/null 2>&1; then
                found=$((found+1))
            else
                echo "- [ ] \`$f\` is missing or heavily refactored."
                missing=$((missing+1))
            fi
        fi
    done
    
    echo ""
    echo "Found $found mapped functions. Missing/Refactored: $missing."
    echo ""
}

echo "# Comprehensive C-to-Go Parity Audit" > "$GO_DIR/docs/NEW_PARITY_TODO.md"
echo "Auto-generated analysis comparing the original Ironwail C codebase to the Go port." >> "$GO_DIR/docs/NEW_PARITY_TODO.md"
echo "" >> "$GO_DIR/docs/NEW_PARITY_TODO.md"

check_file "sv_phys.c" "server" >> "$GO_DIR/docs/NEW_PARITY_TODO.md"
check_file "sv_move.c" "server" >> "$GO_DIR/docs/NEW_PARITY_TODO.md"
check_file "sv_user.c" "server" >> "$GO_DIR/docs/NEW_PARITY_TODO.md"
check_file "sv_main.c" "server" >> "$GO_DIR/docs/NEW_PARITY_TODO.md"

check_file "cl_main.c" "client" >> "$GO_DIR/docs/NEW_PARITY_TODO.md"
check_file "cl_parse.c" "client" >> "$GO_DIR/docs/NEW_PARITY_TODO.md"
check_file "cl_input.c" "client" >> "$GO_DIR/docs/NEW_PARITY_TODO.md"
check_file "cl_tent.c" "client" >> "$GO_DIR/docs/NEW_PARITY_TODO.md"

check_file "host.c" "host" >> "$GO_DIR/docs/NEW_PARITY_TODO.md"
check_file "host_cmd.c" "host" >> "$GO_DIR/docs/NEW_PARITY_TODO.md"

check_file "net_main.c" "net" >> "$GO_DIR/docs/NEW_PARITY_TODO.md"

check_file "snd_dma.c" "audio" >> "$GO_DIR/docs/NEW_PARITY_TODO.md"
check_file "snd_mix.c" "audio" >> "$GO_DIR/docs/NEW_PARITY_TODO.md"

check_file "r_world.c" "renderer" >> "$GO_DIR/docs/NEW_PARITY_TODO.md"
check_file "r_alias.c" "renderer" >> "$GO_DIR/docs/NEW_PARITY_TODO.md"
check_file "r_part.c" "renderer" >> "$GO_DIR/docs/NEW_PARITY_TODO.md"

echo "Done."
