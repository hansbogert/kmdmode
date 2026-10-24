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
	//go:embed kmdmode.plugin.zsh
	pluginZsh string
	//go:embed kview
	kviewZsh string
	// The licenses of everything compiled in, kmdmode's own and those of fzf and its dependencies,
	// whose terms require shipping them with the binary; `make THIRD_PARTY_LICENSES` regenerates
	// the latter from the module cache.
	//go:embed LICENSE
	licenseText string
	//go:embed THIRD_PARTY_LICENSES
	thirdPartyLicenses string
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
// re-sources `kmdmode init zsh` on its next Alt-m, so an upgrade reaches shells already open.
func glueVersion() string {
	sum := sha256.Sum256([]byte(pluginZsh + kviewZsh))
	return hex.EncodeToString(sum[:6])
}

func executable() string {
	exe, err := os.Executable()
	if err != nil {
		return "kmdmode"
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		return resolved
	}
	return exe
}

// initZsh prints the plugin for `source <(kmdmode init zsh)`, preset with this binary's path and
// the glue version.
func initZsh() {
	fmt.Printf("typeset -g _kmdmode_bin=%s _kmdmode_glue=%s\n", shellQuote(executable()), glueVersion())
	fmt.Print(pluginZsh)
}

// shellQuote quotes s for zsh: single quotes, so nothing in it (a $ in a path, say) expands.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// runKview runs the embedded kview script under zsh, telling it where this binary is (for its
// built-in fzf and helpers).
func runKview(args []string) error {
	zsh, err := exec.LookPath("zsh")
	if err != nil {
		return fmt.Errorf("kview needs zsh: %w", err)
	}
	env := append(os.Environ(), "KMDMODE_BIN="+executable())
	argv := append([]string{"zsh", "-c", kviewZsh, "kview"}, args...)
	return syscall.Exec(zsh, argv, env)
}
