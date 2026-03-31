BIN      := drata
MODULE   := github.com/sderosiaux/drata-cli
LDFLAGS  := -s -w

.PHONY: build install lint setup clean

build:
	go build -ldflags "$(LDFLAGS)" -o $(BIN) .

install:
	go install -ldflags "$(LDFLAGS)" .

lint:
	golangci-lint run ./...

setup:
	git config core.hooksPath .githooks

clean:
	rm -f $(BIN)
