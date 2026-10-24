package main

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"syscall"
)

// The zsh side ships inside the binary, as `kubectl completion zsh` does, so a `go install` is the
// whole install and the glue always matches the binary that answers it.
var (
	//go:embed cmdmode.plugin.zsh
	pluginZsh string
	//go:embed kview
	kviewZsh string
)

// version is set by the release build (-ldflags "-X main.version=..."); a `go install ...@vX`
// build reports its module version instead.
var version = ""

func releaseVersion() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
		return info.Main.Version
	}
	return "devel"
}

// glueVersion identifies the embedded zsh code. A shell whose loaded plugin reports another one
// re-sources `cmdmode init zsh` on its next Alt-m, so an upgrade reaches shells already open.
func glueVersion() string {
	sum := sha256.Sum256([]byte(pluginZsh + kviewZsh))
	return hex.EncodeToString(sum[:6])
}

func executable() string {
	exe, err := os.Executable()
	if err != nil {
		return "cmdmode"
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		return resolved
	}
	return exe
}

// initZsh prints the plugin for `source <(cmdmode init zsh)`, preset with this binary's path and
// the glue version.
func initZsh() {
	fmt.Printf("typeset -g _cmdmode_bin=%s _cmdmode_glue=%s\n", shellQuote(executable()), glueVersion())
	fmt.Print(pluginZsh)
}

// shellQuote quotes s for zsh: single quotes, so nothing in it (a $ in a path, say) expands.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// runKview runs the embedded kview script under zsh, telling it where this binary is, and the fzf
// next to it if there is one (a dev checkout's bin/fzf); otherwise kview takes fzf from PATH.
func runKview(args []string) error {
	zsh, err := exec.LookPath("zsh")
	if err != nil {
		return fmt.Errorf("kview needs zsh: %w", err)
	}
	exe := executable()
	env := append(os.Environ(), "CMDMODE_BIN="+exe)
	if fzf := filepath.Join(filepath.Dir(exe), "fzf"); os.Getenv("CMDMODE_FZF") == "" {
		if st, err := os.Stat(fzf); err == nil && !st.IsDir() {
			env = append(env, "CMDMODE_FZF="+fzf)
		}
	}
	argv := append([]string{"zsh", "-c", kviewZsh, "kview"}, args...)
	return syscall.Exec(zsh, argv, env)
}
