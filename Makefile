bin/kmdmode: *.go go.mod kmdmode.plugin.zsh kview
	go build -o $@ .

test: bin/kmdmode bin/fzf
	go test ./...
	./test.zsh

# kview's tests need fzf 0.59+, newer than some distributions ship; this one is for the checkout
# alone, not put on PATH.
bin/fzf:
	GOBIN=$(CURDIR)/bin go install github.com/junegunn/fzf@v0.74.4

install:
	go install .

.PHONY: test install
