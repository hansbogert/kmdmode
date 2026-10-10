bin/cmdmode: *.go go.mod
	go build -o $@ .

test: bin/cmdmode
	go test ./...
	./test.zsh

install: bin/cmdmode
	ln -sf $(CURDIR)/kview $(HOME)/.local/bin/kview

.PHONY: test install
