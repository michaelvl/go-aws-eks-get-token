APP=go-aws-eks-get-token

.PHONY: build lint clean

build:
	CGO_ENABLED=0 go build -o $(APP)

lint:
	golangci-lint run

clean:
	rm -f $(APP)
