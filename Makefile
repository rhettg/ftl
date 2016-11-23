.PHONY: test all clean

all: build/ftl

build:
	mkdir build

clean:
	rm -rf build
	rm -rf test-root

build/ftl: build ftl.go ftl/*.go
	go build -o build/ftl .

test:
	go test .

full-test: test build/ftl
	FTL=build/ftl run-parts --regex=\d*_.*.sh --exit-on-error tests
