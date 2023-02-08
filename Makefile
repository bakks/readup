bin/readup: main.go
	mkdir -p bin
	go build -o bin/readup ./...

install: bin/readup
	cp bin/* ~/bin/

clean:
	rm -rf bin

all: bin/readup

.PHONY: install clean all

