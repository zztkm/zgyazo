.PHONY: build
build:
	go build -ldflags="-H=windowsgui" -o zgyazo.exe

.PHONY: install
install:
	go install -ldflags="-H=windowsgui" .

.PHONY: dist
dist:
	@if not exist dist mkdir dist
	go build -trimpath -ldflags="-s -w -H=windowsgui" -o dist/zgyazo.exe

.PHONY: installer
installer: dist
	powershell -ExecutionPolicy Bypass -File scripts/build_windows.ps1

.PHONY: installer-no-sign
installer-no-sign: dist
	powershell -ExecutionPolicy Bypass -File scripts/build_windows.ps1 -SkipSigning

.PHONY: release
release:
	powershell -ExecutionPolicy Bypass -File scripts/build_windows.ps1 -Version $(VERSION)

.PHONY: clean
clean:
	@if exist dist rmdir /s /q dist
	@if exist zgyazo.exe del zgyazo.exe