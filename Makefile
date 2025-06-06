.PHONY: build
build:
	GOOS=windows go build -ldflags="-H=windowsgui" -o zgyazo.exe

.PHONY: install
install:
	GOOS=windows go install -ldflags="-H=windowsgui" .

.PHONY: dist
dist:
	mkdir dist
	GOOS=windows go build -trimpath -ldflags="-s -w -H=windowsgui" -o dist/zgyazo.exe

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
	rm -rf dist
	rm -f zgyazo.exe