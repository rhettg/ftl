.PHONY: test all clean

all: build/ftl

build:
	mkdir build

clean:
	rm -rf build
	rm -rf test-root

build/ftl: build
	go build -o build/ftl .

test: build/ftl
	mkdir -p test-root
	FTL=build/ftl FTL_ROOT=test-root run-parts --regex=\d*_.*.sh --exit-on-error tests
