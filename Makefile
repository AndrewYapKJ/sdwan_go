VERSION = v1.2.0
LDFLAGS = -ldflags="-s -w"

all: hub-linux-amd64 hub-linux-arm64 cpe-linux-amd64 cpe-linux-arm64

hub-linux-%:
	CGO_ENABLED=0 GOOS=linux GOARCH=$* go build $(LDFLAGS) -o bin/hub-linux-$* ./hub

cpe-linux-%:
	CGO_ENABLED=0 GOOS=linux GOARCH=$* go build $(LDFLAGS) -o bin/cpe-linux-$* ./cpe

clean:
	rm -rf bin/*
