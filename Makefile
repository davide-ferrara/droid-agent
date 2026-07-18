.PHONY: build run test log install clean

build:
	go build -o droid .

run: build
	./droid

test:
	go test ./...

log:
	tail -f /tmp/droid.log

install: build
	cp droid /usr/local/bin/

clean:
	rm -f droid main
