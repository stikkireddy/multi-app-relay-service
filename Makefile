clean:
	@echo "Cleaning up..."
	@rm -f mars

build-mac:
	@echo "Building for Mac"
	@go build -o mars main.go

build-linux:
	@echo "Building for Linux"
	# minimize size by removing a bunch of linker details for debugging, there is a limit of 10mb
	@GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -gcflags=all=-l -o mars main.go