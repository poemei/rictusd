APP := rictusd
PKG := rictusd/cmd/rictusd
OUT := bin/$(APP)
CTL := rictusctl
CTL_PKG := rictusd/cmd/rictusctl
CTL_OUT := bin/$(CTL)

all: build

build:
	@mkdir -p bin
	@go build -trimpath -ldflags="-s -w" -o $(OUT) $(PKG)
	@echo "built $(OUT)"
	
	@go build -trimpath -ldflags="-s -w" -o $(CTL_OUT) $(CTL_PKG)
	@echo "built $(CTL_OUT)"

run: build
	@./bin/$(APP) -config data/rictus.json

clean:
	@rm -rf bin

build-linux-amd64:
	@mkdir -p bin
	@GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o bin/$(APP)-linux-amd64 $(PKG)

build-linux-arm64:
	@mkdir -p bin
	@GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o bin/$(APP)-linux-arm64 $(PKG)
