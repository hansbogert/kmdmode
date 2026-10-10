bin/cmdmode: *.go go.mod
	go build -o $@ .

test: bin/cmdmode bin/fzf
	go test ./...
	./test.zsh

# kview needs a newer fzf than distros ship; this one is for kview alone, not put on PATH.
bin/fzf:
	GOBIN=$(CURDIR)/bin go install github.com/junegunn/fzf@v0.74.4

install: bin/cmdmode bin/fzf
	ln -sf $(CURDIR)/kview $(HOME)/.local/bin/kview

.PHONY: test install
