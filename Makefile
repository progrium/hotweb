
install:
	go install ./cmd/hotweb

test:
	go test -v ./pkg/...