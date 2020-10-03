all: nature-remo-exporter

nature-remo-exporter: nature-remo-exporter.go
	go build -o $@ $^
