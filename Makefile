.PHONY: build install

build: ./main.go
	go build

install: ./main.go
	go install
