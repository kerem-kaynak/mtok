# mtok demo — render from the repo root: vhs docs/demo.tape
# (https://github.com/charmbracelet/vhs)
#
# The hidden setup stages synthetic logs with docs/demo-data.py — the GIF
# must never show real project names or spend — and builds the binary into
# the staged dir. The alias runs with --no-cache: pointing a cached scan at
# the demo dirs would clobber ~/.cache/mtok/scan.gob and its retention rows.

Output docs/demo.gif

Set Shell zsh
Set FontSize 15
Set Width 1400
Set Height 960
Set Padding 12
Set TypingSpeed 55ms
Set Framerate 24

Hide
Type "cd ~/mtok && python3 docs/demo-data.py && go build -o /tmp/mtok-demo/mtok ./cmd/mtok && alias mtok='/tmp/mtok-demo/mtok --no-cache --claude-dir /tmp/mtok-demo/claude --codex-dir /tmp/mtok-demo/codex' && cd /tmp/mtok-demo && PROMPT='%F{magenta}❯%f ' && clear"
Enter
Sleep 15s
Show

# --- dashboard: the hero shot ------------------------------------------------
Sleep 500ms
Type "mtok"
Sleep 400ms
Enter
Sleep 4.5s

# --- t flips every chart and KPI between cost and raw tokens ------------------
Type "t"
Sleep 3s
Type "t"
Sleep 1.5s

# --- daily drill-down: the detail pane follows the cursor ---------------------
Type "2"
Sleep 2s
Down
Sleep 400ms
Down
Sleep 400ms
Down
Sleep 2s

# --- monthly -------------------------------------------------------------------
Type "3"
Sleep 2s
Down
Sleep 400ms
Down
Sleep 2s

# --- models: cost, share, token breakdown, hit rate ---------------------------
Type "4"
Sleep 3.5s

# --- projects: spend per working directory ------------------------------------
Type "5"
Sleep 3s

# --- sessions ------------------------------------------------------------------
Type "6"
Sleep 1.5s
Down
Sleep 300ms
Down
Sleep 2s

# --- back to the dashboard, then out ------------------------------------------
Type "1"
Sleep 2.5s
Type "q"
Sleep 800ms
