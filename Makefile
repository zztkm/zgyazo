.PHONY: build
build:
	go build -ldflags="-H=windowsgui" -o zgyazo.exe

.PHONY: install
install:
	go install -ldflags="-H=windowsgui" .