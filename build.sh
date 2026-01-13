GOARCH=arm GOARM=7 GOOS=linux go build -o bin/armv7l ./cmd/client 
GOARCH=arm64 GOARM="" GOOS=linux go build -o bin/aarch64 ./cmd/client

# $env:GOARCH = "arm"; $env:GOARM = "7"; $env:GOOS = "linux"; go build -o bin/armv7l ./cmd/client
# $env:GOARCH = "arm64"; $env:GOARM = ""; $env:GOOS = "linux"; go build -o bin/aarch64 ./cmd/client
