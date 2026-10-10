bin/cmdmode: *.go go.mod
	go build -o $@ .

test: bin/cmdmode
	go test ./...
	./test.zsh

.PHONY: test
