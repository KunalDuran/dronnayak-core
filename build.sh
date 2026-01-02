GOARCH=arm GOARM=7 GOOS=linux go build -o bin/armv7l ./cmd/client 
GOARCH=arm64 GOARM="" GOOS=linux go build -o bin/aarch64 ./cmd/client 
