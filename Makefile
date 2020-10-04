COMMIT_HASH = $(shell git rev-list -n1 HEAD)
VERSION_POSTFIX = $(shell git diff --quiet --exit-code && echo "" || echo "+")
VERSION = ${COMMIT_HASH}${VERSION_POSTFIX}

all: nature-remo-exporter

nature-remo-exporter: nature-remo-exporter.go
	go build -ldflags "-X main.version=${VERSION}" -o $@ $^

clean:
	rm -f nature-remo-exporter
