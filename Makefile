BIN      := drata
MODULE   := github.com/sderosiaux/drata-cli
LDFLAGS  := -s -w

.PHONY: build install lint clean

build:
	go build -ldflags "$(LDFLAGS)" -o $(BIN) .

install:
	go install -ldflags "$(LDFLAGS)" .

lint:
	go vet ./...

clean:
	rm -f $(BIN)
