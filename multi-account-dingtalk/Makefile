.PHONY: all build

all: build

build: 
	if [ -d eventHandler/bin ]; then rm -rf eventHandler/bin/*; fi;
	GOARCH=amd64 GOOS=linux go build -o eventHandler/bin/main eventHandler/main.go
	zip eventHandler/bin/main.zip eventHandler/bin/main