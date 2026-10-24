bin/kmdmode: *.go go.mod go.sum kmdmode.plugin.zsh kview LICENSE THIRD_PARTY_LICENSES
	go build -o $@ .

test: bin/kmdmode
	go test ./...
	./test.zsh

# The license of every module compiled into the binary (fzf and its dependencies), embedded and
# shipped with releases, as their terms require. Regenerate after changing dependencies.
THIRD_PARTY_LICENSES: go.mod go.sum
	for os in linux darwin; do for arch in amd64 arm64; do \
	  GOOS=$$os GOARCH=$$arch go list -deps \
	    -f '{{with .Module}}{{if not .Main}}{{.Path}} {{.Version}} {{.Dir}}{{end}}{{end}}' .; \
	done; done | sort -u | while read -r path version dir; do \
	  printf '================================================================================\n'; \
	  printf '%s %s\n\n' "$$path" "$$version"; \
	  found=; for f in "$$dir"/LICENSE* "$$dir"/COPYING*; do \
	    [ -f "$$f" ] && cat "$$f" && found=1; \
	  done; [ -n "$$found" ] || echo "(no license file found)"; \
	  printf '\n'; \
	done > $@

install:
	go install .

.PHONY: test install
