VERSION=v0.0.1

.PHONY: stackver
stackver: clean bin/stackver_darwin_amd64 bin/stackver_darwin_arm64 
stackver: bin/stackver_linux_amd64 bin/stackver_linux_arm64 bin/stackver_hostarch 
stackver: bin/stackver_windows_amd64 bin/stackver_windows_arm64

clean:
	rm -rf bin

bin/stackver_darwin_amd64:
	mkdir -p bin
	GOOS=darwin GOARCH=amd64 go build -ldflags="-X 'main.Version=$(VERSION)'" -o bin/stackver_darwin_amd64 cmd/stackver/*.go
	openssl sha512 bin/stackver_darwin_amd64 > bin/stackver_darwin_amd64.sha512

bin/stackver_darwin_arm64:
	mkdir -p bin
	GOOS=darwin GOARCH=arm64 go build -ldflags="-X 'main.Version=$(VERSION)'" -o bin/stackver_darwin_arm64 cmd/stackver/*.go
	openssl sha512 bin/stackver_darwin_arm64 > bin/stackver_darwin_arm64.sha512

bin/stackver_linux_amd64:
	mkdir -p bin
	GOOS=linux GOARCH=amd64 go build -ldflags="-X 'main.Version=$(VERSION)'" -o bin/stackver_linux_amd64 cmd/stackver/*.go
	openssl sha512 bin/stackver_linux_amd64 > bin/stackver_linux_amd64.sha512

bin/stackver_linux_arm64:
	mkdir -p bin
	GOOS=linux GOARCH=arm64 go build -ldflags="-X 'main.Version=$(VERSION)'" -o bin/stackver_linux_arm64 cmd/stackver/*.go
	openssl sha512 bin/stackver_linux_arm64 > bin/stackver_linux_arm64.sha512

bin/stackver_hostarch:
	mkdir -p bin
	go build -ldflags="-X 'main.Version=$(VERSION)'" -o bin/stackver_hostarch cmd/stackver/*.go
	openssl sha512 bin/stackver_hostarch > bin/stackver_hostarch.sha512

bin/stackver_windows_amd64:
	mkdir -p bin
	GOOS=windows GOARCH=amd64 go build -ldflags="-X 'main.Version=$(VERSION)'" -o bin/stackver_windows_amd64 cmd/stackver/*.go
	openssl sha512 bin/stackver_windows_amd64 > bin/stackver_windows_amd64.sha512

bin/stackver_windows_arm64:
	mkdir -p bin
	GOOS=windows GOARCH=arm64 go build -ldflags="-X 'main.Version=$(VERSION)'" -o bin/stackver_windows_arm64 cmd/stackver/*.go
	openssl sha512 bin/stackver_windows_arm64 > bin/stackver_windows_arm64.sha512


robertlestak/stackver:
	docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7 \
	-t robertlestak/stackver:latest -t robertlestak/stackver:$(VERSION) \
	--push .