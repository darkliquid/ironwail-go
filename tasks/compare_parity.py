import os
import re

C_DIR = "/home/darkliquid/Projects/ironwail/Quake"
GO_DIR = "/home/darkliquid/Projects/ironwail-go"

c_files = [
    "sv_phys.c", "sv_move.c", "sv_user.c", "sv_main.c",
    "cl_main.c", "cl_parse.c", "cl_input.c", "cl_tent.c",
    "host.c", "host_cmd.c", "net_main.c", "net_dgrm.c",
    "snd_dma.c", "snd_mix.c", "r_world.c", "r_alias.c", "r_part.c"
]

go_functions = set()
for root, _, files in os.walk(GO_DIR):
    if ".git" in root or ".beads" in root:
        continue
    for file in files:
        if file.endswith(".go"):
            with open(os.path.join(root, file), 'r', encoding='utf-8') as f:
                content = f.read()
                # Find func Name(...) or func (r *Recv) Name(...)
                matches = re.findall(r'func\s+(?:\([^)]+\)\s+)?([a-zA-Z0-9_]+)\s*\(', content)
                for m in matches:
                    go_functions.add(m.lower())

with open(os.path.join(GO_DIR, "docs", "NEW_PARITY_TODO.md"), "w") as out:
    out.write("# Comprehensive C-to-Go Parity Audit\n\n")
    out.write("Auto-generated analysis comparing the original Ironwail C codebase to the Go port.\n\n")

    for c_file in c_files:
        out.write(f"### Analysis of `{c_file}`\n\n")
        c_path = os.path.join(C_DIR, c_file)
        if not os.path.exists(c_path):
            out.write(f"File not found: {c_file}\n\n")
            continue
            
        with open(c_path, 'r', encoding='utf-8', errors='ignore') as f:
            content = f.read()
            # Basic C function regex: return_type func_name(args) {
            matches = re.findall(r'^[a-zA-Z_][a-zA-Z0-9_*\s]+?\s+([a-zA-Z0-9_]+)\s*\([^;]*$', content, re.MULTILINE)
            
            missing = []
            found = 0
            
            for m in matches:
                # Strip prefixes and common suffixes
                search_name = re.sub(r'^(SV_|CL_|Net_|R_|S_|Host_|Sys_|Q_)', '', m, flags=re.IGNORECASE)
                search_name = search_name.replace("_", "").lower()
                
                # Check against collected Go functions
                match_found = False
                for go_func in go_functions:
                    if search_name in go_func.lower():
                        match_found = True
                        break
                        
                if match_found:
                    found += 1
                else:
                    missing.append(m)
            
            if missing:
                for msg in sorted(set(missing)):
                    if msg not in ["if", "while", "for", "switch", "return"]:
                        out.write(f"- [ ] `{msg}` is missing or heavily refactored.\n")
            out.write(f"\nFound {found} mapped functions. Missing: {len(missing)}.\n\n")

print("Done.")
