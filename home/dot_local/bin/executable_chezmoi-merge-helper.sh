#!/usr/bin/env bash
# Helper script for chezmoi merge with clear labels
# Arguments: $1=destination $2=target $3=source

destination="$1"
target="$2"
source="$3"

# Create base file (copy of target state)
base="${target}.base"
cp "$target" "$base"

# Show clear explanation before merge
cat << EOF
╔════════════════════════════════════════════════════════════════╗
║                    CHEZMOI MERGE LAYOUT                        ║
╠════════════════════════════════════════════════════════════════╣
║                                                                ║
║  LEFT (Current):    DESTINATION - File in your home dir       ║
║                     ↓ External changes (e.g., app updates)    ║
║                                                                ║
║  RIGHT (Incoming):  TARGET - What chezmoi wants to apply      ║
║                     ↓ Your chezmoi changes (templates, etc)   ║
║                                                                ║
║  BOTTOM (Result):   SOURCE - Will be saved to chezmoi repo    ║
║                                                                ║
╠════════════════════════════════════════════════════════════════╣
║  File: $(basename "$destination")
╚════════════════════════════════════════════════════════════════╝

Press Enter to open merge editor...
EOF

read -r

# Launch VSCode merge
# Syntax: code --merge <current> <incoming> <base> <result>
code --new-window --wait --merge "$destination" "$target" "$base" "$source"

# Cleanup
rm -f "$base"
